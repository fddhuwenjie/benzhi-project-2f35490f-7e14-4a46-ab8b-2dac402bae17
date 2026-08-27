const state = { id: localStorage.getItem('sheltergate.drill') || '', view: null, preview: null };
const $ = selector => document.querySelector(selector);
const stageLabels = [['draft', '建档'], ['baseline_frozen', '基线'], ['executing', '检查'], ['remediation', '整改'], ['ready_for_review', '送审'], ['activated', '决定']];
const eventLabels = { drill_created: '创建演练档案', drill_revised: '修订草稿资料', baseline_frozen: '冻结布设基线', checkpoint_recorded: '记录现场检查', deviation_remediated: '提交偏差整改材料', checkpoint_retested: '执行定向复验', review_submitted: '提交独立复核', review_decided: '形成复核结论', review_items_responded: '回应复核退回事项' };

function requestID() { return 'web_' + crypto.randomUUID(); }
function notice(message, error = false) { const node = $('#notice'); node.textContent = message; node.className = 'notice' + (error ? ' error' : ''); node.hidden = false; setTimeout(() => node.hidden = true, 5000); }
async function api(path, options = {}) { const response = await fetch(path, options); let body; try { body = await response.json(); } catch { body = {}; } if (!response.ok) { const details = body.error?.details?.map(item => item.message).join('；'); throw new Error(details || body.error?.message || `请求失败 (${response.status})`); } return body; }
function command(body = {}) { return { ...body, request_id: requestID(), expected_version: state.view?.aggregate.drill.version || 0 }; }
function lines(value) { return value.split('\n').map(item => item.trim()).filter(Boolean); }
function isoLocal(value) {
  const date = new Date(value), offset = -date.getTimezoneOffset(), sign = offset >= 0 ? '+' : '-';
  const hours = String(Math.floor(Math.abs(offset) / 60)).padStart(2, '0'), minutes = String(Math.abs(offset) % 60).padStart(2, '0');
  return `${value}:00${sign}${hours}:${minutes}`;
}
function escapeHTML(value) { return String(value).replace(/[&<>'"]/g, char => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;' }[char])); }
function statusLabel(status) { return { draft: '草稿', baseline_frozen: '基线已冻结', executing: '检查执行中', remediation: '整改中', ready_for_review: '待送审', under_review: '独立复核中', returned: '复核退回', activated: '已批准启用' }[status] || status; }

async function loadList() {
  const data = await api('/api/drills');
  const select = $('#drill-select');
  select.innerHTML = '<option value="">新建演练</option>' + data.drills.map(drill => `<option value="${drill.id}">${escapeHTML(drill.site_name)} · ${statusLabel(drill.status)}</option>`).join('');
  select.value = state.id;
}
async function load() {
  await loadList();
  if (!state.id) { state.view = null; render(); return; }
  try { state.view = await api(`/api/drills/${state.id}`); render(); }
  catch (error) { state.id = ''; localStorage.removeItem('sheltergate.drill'); notice(error.message, true); render(); }
}

function render() {
  const view = state.view;
  $('#create-panel').hidden = !!view;
  ['baseline-panel', 'timeline-panel'].forEach(id => $('#' + id).hidden = !view);
  if (!view) { $('#site-title').textContent = '新建演练档案'; $('#site-meta').textContent = '登记基本资料后开始布设核验'; $('#edit-drill-btn').hidden = true; $('#progress-panel').hidden = true; renderSteps('draft'); return; }
  const aggregate = view.aggregate, drill = aggregate.drill;
  $('#site-title').textContent = drill.site_name;
  $('#site-meta').textContent = `容量 ${drill.planned_capacity} 人 · 负责人 ${drill.lead_name} · ${drill.scheduled_date} · v${drill.version}`;
  $('#edit-drill-btn').hidden = drill.status !== 'draft';
  renderSteps(drill.status);
  $('#baseline-form').hidden = drill.status !== 'draft';
  $('#baseline-readonly').hidden = drill.status === 'draft';
  if (drill.status !== 'draft') renderBaseline(aggregate.baseline);
  $('#progress-panel').hidden = !aggregate.checkpoints.length;
  if (aggregate.checkpoints.length) renderProgress(view.progress);
  $('#checks-panel').hidden = !aggregate.checkpoints.length;
  renderChecks(aggregate);
  $('#deviation-panel').hidden = !aggregate.deviations.length;
  renderDeviations(aggregate);
  const reviewVisible = ['ready_for_review', 'under_review', 'returned', 'activated'].includes(drill.status);
  $('#review-panel').hidden = !reviewVisible;
  $('#submit-review-box').hidden = !['ready_for_review', 'returned'].includes(drill.status);
  $('#review-form').hidden = drill.status !== 'under_review';
  $('#returned-items').hidden = drill.status !== 'returned';
  if (drill.status === 'returned') renderReturnedItems(aggregate);
  $('#decision-panel').hidden = !aggregate.decision;
  if (aggregate.decision) renderDecision(view);
  renderTimeline(view);
}

function renderSteps(status) {
  const normalized = ['returned', 'under_review'].includes(status) ? 'ready_for_review' : status;
  let current = Math.max(0, stageLabels.findIndex(item => item[0] === normalized));
  if (['baseline_frozen', 'executing', 'remediation'].includes(status)) current = { baseline_frozen: 1, executing: 2, remediation: 3 }[status];
  $('#status-steps').innerHTML = stageLabels.map((item, index) => `<li class="${index < current ? 'done' : index === current ? 'active' : ''}">${item[1]}</li>`).join('');
}
function renderProgress(progress) {
  const values = [['完成率', `${progress.completion_percent}%`], ['初次检查', progress.initial_completed], ['通过', progress.passed], ['失败', progress.failed], ['待整改', progress.open_deviations], ['已关闭', progress.closed_deviations]];
  $('#progress-stats').innerHTML = values.map(([label, value]) => `<div class="stat"><strong>${value}</strong><span>${label}</span></div>`).join('');
  $('#next-action').textContent = progress.next_action;
  $('#rule-summary').innerHTML = progress.rule_counts.length ? progress.rule_counts.map(rule => `<span class="tag open">${escapeHTML(rule.rule_code)} · ${rule.count}</span>`).join('') : '<span class="tag closed">无开放规则偏差</span>';
}
function renderBaseline(baseline) {
  const groups = [['疏散入口', baseline.entrances], ['疏散路径', baseline.evacuation_routes], ['功能分区', baseline.functional_zones], ['关键设施', baseline.critical_facilities]];
  $('#baseline-summary').textContent = `基线 v${baseline.version} · 摘要 ${baseline.content_digest.slice(0, 12)}…`;
  $('#baseline-readonly').innerHTML = groups.map(([name, items]) => `<article><h3>${name}</h3><ul>${items.map(item => `<li>${escapeHTML(item)}</li>`).join('')}</ul></article>`).join('');
}
function renderChecks(aggregate) {
  $('#checkpoint-body').innerHTML = aggregate.checkpoints.map(cp => {
    const results = aggregate.results.filter(result => result.checkpoint_code === cp.code), last = results.at(-1), initial = results.find(result => result.attempt === 1);
    const canRecord = !initial && ['baseline_frozen', 'executing', 'remediation'].includes(aggregate.drill.status);
    return `<tr><td>${cp.order}</td><td><span class="checkpoint-name">${escapeHTML(cp.name)}</span><span class="subtle">${escapeHTML(cp.requirement)}</span></td><td>≤ ${cp.max_seconds} 秒<br><span class="subtle">需证据摘要</span></td><td>${last ? `<span class="tag ${last.outcome}">${last.outcome === 'pass' ? '通过' : '不通过'} · 第 ${last.attempt} 次</span><span class="subtle">${last.measured_seconds} 秒 / ${last.participant_count} 人</span>` : '<span class="tag">未执行</span>'}</td><td><button data-check="${cp.code}" ${canRecord ? '' : 'disabled'}>记录</button></td></tr>`;
  }).join('');
  document.querySelectorAll('[data-check]').forEach(button => button.onclick = () => openCheck(button.dataset.check));
}
function renderDeviations(aggregate) {
  $('#deviation-list').innerHTML = aggregate.deviations.map(deviation => {
    const materials = aggregate.remediation_materials?.filter(item => item.deviation_id === deviation.id) || [];
    const attempts = aggregate.retest_attempts?.filter(item => item.deviation_id === deviation.id) || [];
    const history = materials.map(material => `<span class="subtle">材料 v${material.version} · ${escapeHTML(material.evidence_digest)}</span>`).join('') + attempts.map(attempt => `<span class="subtle">复验 ${attempt.attempt} · 材料 v${attempt.material_version} · ${attempt.passed ? '通过' : escapeHTML(attempt.failure_reason)}</span>`).join('');
    const status = deviation.status === 'closed' ? '已关闭' : deviation.status === 'ready_for_retest' ? '待复验' : '待整改';
    const actions = deviation.status === 'closed' ? '' : `<button data-remediate="${deviation.id}">${deviation.material_version ? '提交新版材料' : '提交整改'}</button>${deviation.status === 'ready_for_retest' ? `<button class="primary" data-retest="${deviation.id}" data-code="${deviation.checkpoint_code}">定向复验</button>` : ''}`;
    return `<article class="deviation"><div><h3>${checkpointName(aggregate, deviation.checkpoint_code)} <span class="tag ${deviation.status === 'closed' ? 'closed' : 'open'}">${status}</span></h3><p>规则：${escapeHTML(deviation.rule_code)}</p>${deviation.cause ? `<p>原因：${escapeHTML(deviation.cause)} · 措施：${escapeHTML(deviation.corrective_action)}</p>` : ''}${history}</div><div>${actions}</div></article>`;
  }).join('');
  document.querySelectorAll('[data-remediate]').forEach(button => button.onclick = () => openRemediation(button.dataset.remediate));
  document.querySelectorAll('[data-retest]').forEach(button => button.onclick = () => openCheck(button.dataset.code, button.dataset.retest));
}
function checkpointName(aggregate, code) { return aggregate.checkpoints.find(cp => cp.code === code)?.name || code; }
function renderReturnedItems(aggregate) {
  const round = aggregate.review_rounds.at(-1);
  $('#returned-items').innerHTML = `<h3>第 ${round.round} 轮退回事项</h3><form id="response-form">${round.items.map(item => `<div class="review-response"><h3>${escapeHTML(item.description)}</h3><p>${escapeHTML(item.requirement)}</p><input type="hidden" name="item_id" value="${item.id}"><label>回应<textarea name="response" required>${escapeHTML(item.response.response || '')}</textarea></label><label>证据摘要<input name="evidence_digest" value="${escapeHTML(item.response.evidence_digest || '')}" required></label></div>`).join('')}<div class="form-action"><button class="primary" type="submit">保存全部回应</button></div></form>`;
  $('#response-form').onsubmit = submitReviewResponses;
}
function renderDecision(view) {
  const decision = view.aggregate.decision, doc = view.decision_document;
  const rows = [['文档类型', doc.document_type], ['演练编号', doc.drill_id], ['场所', doc.site_name], ['演练日期', doc.scheduled_date], ['计划容量', `${doc.planned_capacity} 人`], ['基线版本', `v${doc.baseline_version}`], ['基线摘要', doc.baseline_digest], ['决定', doc.decision], ['复核员', doc.reviewer_name], ['复核意见', doc.review_note || '同意按冻结基线启用'], ['签发时间', new Date(doc.issued_at).toLocaleString()], ['文档摘要', decision.document_digest]];
  $('#decision-content').innerHTML = rows.map(([key, value]) => `<dt>${key}</dt><dd>${escapeHTML(String(value))}</dd>`).join('');
  $('#decision-verify').textContent = view.decision_valid && view.timeline_valid ? '决定书与审计摘要校验通过' : '决定书摘要校验失败';
  $('#export-decision-link').href = `/api/drills/${state.id}/decision/export`;
}
function renderTimeline(view) {
  $('#timeline-valid').textContent = view.timeline_valid ? '审计摘要链连续有效' : '审计摘要链校验失败';
  $('#timeline-list').innerHTML = view.timeline.map(event => `<li><strong>${eventLabels[event.event_type] || event.event_type}</strong><span class="subtle">${new Date(event.occurred_at).toLocaleString()} · 序号 ${event.sequence}</span><code>${event.current_hash}</code></li>`).join('');
}

function baselinePayload() { const form = new FormData($('#baseline-form')); return { entrances: lines(form.get('entrances')), evacuation_routes: lines(form.get('evacuation_routes')), functional_zones: lines(form.get('functional_zones')), critical_facilities: lines(form.get('critical_facilities')) }; }
async function previewBaseline() {
  try {
    state.preview = await api(`/api/drills/${state.id}/baseline/preview`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(baselinePayload()) });
    const node = $('#baseline-preview'), errors = state.preview.validation_errors || [];
    node.hidden = false; node.className = 'preview-panel' + (errors.length ? ' error' : '');
    node.innerHTML = errors.length ? errors.map(error => `<p>${escapeHTML(error.message)}</p>`).join('') : `<strong>基线 v${state.preview.next_version}</strong><p>摘要 ${state.preview.content_digest}</p><ol>${state.preview.checkpoints.map(cp => `<li>${escapeHTML(cp.name)} · 上限 ${cp.max_seconds} 秒</li>`).join('')}</ol>`;
  } catch (error) { notice(error.message, true); }
}
function openCheck(code, deviation = '') { const form = $('#check-form'); form.reset(); form.checkpoint_code.value = code; form.deviation_id.value = deviation; form.participant_count.value = Math.min(20, state.view.aggregate.drill.planned_capacity); const date = state.view.aggregate.drill.scheduled_date; form.started_at.value = date + 'T09:00'; form.ended_at.value = date + 'T09:01'; $('#check-title').textContent = deviation ? '定向复验' : '记录现场检查'; $('#check-dialog').showModal(); }
function openRemediation(id) { const form = $('#remediation-form'); form.reset(); form.deviation_id.value = id; $('#remediation-dialog').showModal(); }
function addReviewItem() { const row = document.createElement('div'); row.className = 'review-item-row'; row.innerHTML = '<input name="description" required placeholder="问题描述"><select name="reference_type"><option value="checkpoint">检查点</option><option value="baseline_item">基线项目</option></select><input name="reference_value" placeholder="关联项"><input name="requirement" required placeholder="处理要求">'; $('#review-item-list').append(row); }

$('#create-form').onsubmit = async event => { event.preventDefault(); const form = new FormData(event.target); try { const data = await api('/api/drills', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(command({ site_name: form.get('site_name'), planned_capacity: Number(form.get('planned_capacity')), lead_name: form.get('lead_name'), scheduled_date: form.get('scheduled_date') })) }); state.id = data.drill.id; localStorage.setItem('sheltergate.drill', state.id); notice('演练草稿已创建'); await load(); } catch (error) { notice(error.message, true); } };
$('#edit-drill-btn').onclick = () => { const drill = state.view.aggregate.drill, form = $('#edit-form'); form.site_name.value = drill.site_name; form.planned_capacity.value = drill.planned_capacity; form.lead_name.value = drill.lead_name; form.scheduled_date.value = drill.scheduled_date; $('#edit-dialog').showModal(); };
$('#edit-form').addEventListener('submit', async event => { event.preventDefault(); if (event.submitter?.value !== 'submit') { $('#edit-dialog').close(); return; } const form = new FormData(event.target); try { await api(`/api/drills/${state.id}`, { method: 'PATCH', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(command({ site_name: form.get('site_name'), planned_capacity: Number(form.get('planned_capacity')), lead_name: form.get('lead_name'), scheduled_date: form.get('scheduled_date') })) }); $('#edit-dialog').close(); state.preview = null; notice('草稿资料已修订'); await load(); } catch (error) { notice(error.message, true); } });
$('#baseline-preview-btn').onclick = previewBaseline;
$('#baseline-form').onsubmit = async event => { event.preventDefault(); if (!state.preview || state.preview.drill_version !== state.view.aggregate.drill.version) { notice('请先生成当前资料的基线预览', true); return; } try { await api(`/api/drills/${state.id}/baseline/freeze`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(command({ ...baselinePayload(), preview_digest: state.preview.preview_digest })) }); state.preview = null; notice('布设基线已冻结'); await load(); } catch (error) { notice(error.message, true); } };
$('#check-form').addEventListener('submit', async event => { event.preventDefault(); if (event.submitter?.value !== 'submit') { $('#check-dialog').close(); return; } const form = new FormData(event.target), code = form.get('checkpoint_code'), deviation = form.get('deviation_id'), path = deviation ? `/api/drills/${state.id}/deviations/${deviation}/retest` : `/api/drills/${state.id}/checkpoints/${code}/results`; try { await api(path, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(command({ participant_count: Number(form.get('participant_count')), started_at: isoLocal(form.get('started_at')), ended_at: isoLocal(form.get('ended_at')), outcome: form.get('outcome'), evidence_digest: form.get('evidence_digest'), recorded_by: form.get('recorded_by') })) }); $('#check-dialog').close(); notice(deviation ? '定向复验已记录' : '检查结果已记录'); await load(); } catch (error) { notice(error.message, true); } });
$('#remediation-form').addEventListener('submit', async event => { event.preventDefault(); if (event.submitter?.value !== 'submit') { $('#remediation-dialog').close(); return; } const form = new FormData(event.target), id = form.get('deviation_id'); try { await api(`/api/drills/${state.id}/deviations/${id}/remediation`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(command({ cause: form.get('cause'), corrective_action: form.get('corrective_action'), evidence_digest: form.get('evidence_digest') })) }); $('#remediation-dialog').close(); notice('整改材料版本已保存'); await load(); } catch (error) { notice(error.message, true); } });
$('#submit-review-btn').onclick = async () => { try { await api(`/api/drills/${state.id}/review/submit`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(command()) }); notice('已提交独立复核'); await load(); } catch (error) { notice(error.message, true); } };
$('#add-review-item').onclick = addReviewItem;
$('#review-form').onsubmit = async event => { event.preventDefault(); const form = new FormData(event.target), decision = event.submitter?.value; const descriptions = form.getAll('description'), references = form.getAll('reference_type'), values = form.getAll('reference_value'), requirements = form.getAll('requirement'); const items = descriptions.map((description, index) => ({ description, reference_type: references[index], reference_value: values[index], requirement: requirements[index] })); try { await api(`/api/drills/${state.id}/review/decision`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(command({ decision, reviewer_name: form.get('reviewer_name'), review_note: form.get('review_note'), items: decision === 'rejected' ? items : [] })) }); notice(decision === 'approved' ? '启用决定书已生成' : '演练已按事项退回'); await load(); } catch (error) { notice(error.message, true); } };
async function submitReviewResponses(event) { event.preventDefault(); const form = new FormData(event.target), ids = form.getAll('item_id'), responses = form.getAll('response'), evidence = form.getAll('evidence_digest'); try { await api(`/api/drills/${state.id}/review/responses`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(command({ responses: ids.map((item_id, index) => ({ item_id, response: responses[index], evidence_digest: evidence[index] })) })) }); notice('复核事项回应已保存'); await load(); } catch (error) { notice(error.message, true); } }
$('#verify-decision-btn').onclick = async () => { try { const result = await api(`/api/drills/${state.id}/decision/verify`); notice(result.valid ? '决定书、基线和审计时间线校验通过' : result.errors.join('；'), !result.valid); } catch (error) { notice(error.message, true); } };
$('#print-decision-btn').onclick = () => window.print();
$('#drill-select').onchange = event => { state.id = event.target.value; state.preview = null; if (state.id) localStorage.setItem('sheltergate.drill', state.id); else localStorage.removeItem('sheltergate.drill'); load(); };
$('#refresh-btn').onclick = load;
document.querySelectorAll('dialog .icon-btn').forEach(button => button.onclick = event => { event.preventDefault(); button.closest('dialog').close(); });
addReviewItem();
load().catch(error => notice(error.message, true));
