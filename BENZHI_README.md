# BENZHI_README

## 项目说明
- 项目：benzhi-project-2f35490f-7e14-4a46-ab8b-2dac402bae17
- 项目用途：避难场所演练核验系统围绕一次演练固化布设基线、检查点执行、偏差整改、定向复验、独立复核和不可变启用决定，并由 Go 原生单页工作台提供完整操作链路。
- Go 工具链：`golang:1.22`
- 前端工具链：原生 HTML、CSS 和 JavaScript，由 Go 服务直接提供

## 项目描述
- 项目名称：shelter-drill-gate
- 项目介绍：面向社区应急管理人员的避难场所启用演练核验系统，以一次场所演练为唯一业务主线，将布设基线、检查点执行、偏差整改、定向复验和启用决定固化为可追溯档案。
- 项目概述：面向社区应急管理人员的避难场所启用演练核验系统，以一次场所演练为唯一业务主线，将布设基线、检查点执行、偏差整改、定向复验和启用决定固化为可追溯档案。
- 核心工作流：演练负责人创建避难场所演练并冻结布设基线，依次执行疏散入口、分区引导、应急照明和物资可达性检查点；系统汇总观察结果并将不合格项转入整改，负责人提交纠正证据和定向复验结果，全部关闭后由独立安全复核员批准或驳回，批准时生成不可变的启用决定书。
- 对外接口：由 Go 服务直接提供无需 Node 构建链的原生 HTML、CSS 和 JavaScript 单页工作台；页面包含演练状态条、布设基线编辑区、检查点执行表、偏差整改抽屉、复核面板和启用决定书视图，同源 JSON 端点仅服务于该页面。HTTP 监听支持 -addr=127.0.0.1:<port>，默认 127.0.0.1:19081；PORT 为端口号时绑定 127.0.0.1:<PORT>，且不得默认绑定 0.0.0.0。

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...

cd /app && GOTOOLCHAIN=local go run ./cmd/sheltergate -selfcheck -addr=127.0.0.1:19081

cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh

./build_benzhi_docker.sh benzhi-project-2f35490f-7e14-4a46-ab8b-2dac402bae17-amd64 linux/amd64

./build_benzhi_docker.sh benzhi-project-2f35490f-7e14-4a46-ab8b-2dac402bae17-arm64 linux/arm64

docker run -it benzhi-project-2f35490f-7e14-4a46-ab8b-2dac402bae17-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/sheltergate -selfcheck -addr=127.0.0.1:19081`
