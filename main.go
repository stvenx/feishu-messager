package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	webhookBaseURL = "https://open.feishu.cn/open-apis/bot/v2/hook"
)

type FeishuMessage struct {
	MsgType string      `json:"msg_type"`
	Content interface{} `json:"content"`
}

type TextContent struct {
	Text string `json:"text"`
}

type FeishuResponse struct {
	Code int                    `json:"code"`
	Msg  string                 `json:"msg"`
	Data map[string]interface{} `json:"data"`
}

// getEnv 获取环境变量，支持 INPUT_* 格式（GitHub Actions inputs）和直接环境变量格式
func getEnv(key string) string {
	// GitHub Actions inputs 会自动转换为 INPUT_<INPUT_NAME> 格式，名称转换为大写
	// 例如：inputs.bot_token -> INPUT_BOT_TOKEN
	inputKey := "INPUT_" + strings.ToUpper(strings.ReplaceAll(key, "-", "_"))
	if val := os.Getenv(inputKey); val != "" {
		return val
	}
	// 向后兼容：直接使用环境变量（大写格式）
	upperKey := strings.ToUpper(key)
	return os.Getenv(upperKey)
}

func main() {
	// 获取环境变量（支持 INPUT_* 和直接环境变量两种格式）
	botToken := getEnv("bot_token")
	postMessage := getEnv("post_message")
	messageFile := getEnv("message_file")
	msgType := getEnv("msg_type")
	userMaps := getEnv("user_maps")
	assigneesJSON := getEnv("assignees")

	// 验证必需参数
	if botToken == "" {
		fmt.Fprintf(os.Stderr, "::error::Please set the BOT_TOKEN secret.\n")
		os.Exit(1)
	}

	if postMessage == "" && messageFile == "" {
		fmt.Fprintf(os.Stderr, "::error::Please set the post message or a file containing the message.\n")
		os.Exit(1)
	}

	// 处理消息文件
	if postMessage == "" && messageFile != "" {
		content, err := os.ReadFile(messageFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "::error::File '%s' not found: %v\n", messageFile, err)
			os.Exit(1)
		}
		postMessage = string(content)
	}

	// 处理 @用户功能
	if assigneesJSON != "" && userMaps != "" {
		atUsers := parseUsers(userMaps, assigneesJSON)
		if atUsers != "" {
			postMessage = atUsers + postMessage
		}
	}

	// 设置默认消息类型
	if msgType == "" {
		msgType = "text"
	}

	// 验证消息类型
	if msgType != "text" && msgType != "markdown" {
		fmt.Fprintf(os.Stderr, "::error::Unsupported MSG_TYPE: %s. Supported types: text, markdown\n", msgType)
		os.Exit(1)
	}

	// 构建请求体
	var requestBody FeishuMessage
	requestBody.MsgType = "text" // 飞书目前都使用 text 类型，markdown 语法在 text 中支持
	requestBody.Content = TextContent{
		Text: postMessage,
	}

	// 序列化请求体
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		fmt.Fprintf(os.Stderr, "::error::Failed to marshal request body: %v\n", err)
		os.Exit(1)
	}

	// 构建 webhook URL
	webhookURL := fmt.Sprintf("%s/%s", webhookBaseURL, botToken)

	// 调试输出
	fmt.Println("=== Debug: Request Information ===")
	fmt.Printf("URL: %s\n", webhookURL)
	fmt.Println("Method: POST")
	fmt.Println("Content-Type: application/json")
	fmt.Println("Request Body:")
	var prettyJSON bytes.Buffer
	json.Indent(&prettyJSON, jsonData, "", "  ")
	fmt.Println(prettyJSON.String())
	fmt.Println("================================")
	fmt.Println()

	// 发送 HTTP 请求
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequest("POST", webhookURL, strings.NewReader(string(jsonData)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "::error::Failed to create request: %v\n", err)
		os.Exit(1)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "::error::Failed to send request: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	// 读取响应
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "::error::Failed to read response: %v\n", err)
		os.Exit(1)
	}

	// 调试输出响应
	fmt.Println("=== Debug: Response Information ===")
	fmt.Printf("HTTP Status Code: %d\n", resp.StatusCode)
	fmt.Println("Response Body:")
	var prettyResp bytes.Buffer
	json.Indent(&prettyResp, responseBody, "", "  ")
	fmt.Println(prettyResp.String())
	fmt.Println("================================")

	// 解析响应
	var feishuResp FeishuResponse
	if err := json.Unmarshal(responseBody, &feishuResp); err != nil {
		fmt.Fprintf(os.Stderr, "::error::Failed to parse response: %v\n", err)
		os.Exit(1)
	}

	// 检查响应
	if resp.StatusCode != http.StatusOK || feishuResp.Code != 0 {
		fmt.Fprintf(os.Stderr, "::error::Request failed with code %d: %s\n", feishuResp.Code, feishuResp.Msg)
		os.Exit(1)
	}

	fmt.Println("::notice::Message sent successfully")
}

// parseUsers 解析用户映射，生成 @用户的标签
// userMaps 格式：stvenx:ou_xxxx,user1:ou_yyyy
// assigneesJSON 格式：JSON 数组，包含 {"login": "stvenx"} 等对象
func parseUsers(userMaps, assigneesJSON string) string {
	if userMaps == "" || assigneesJSON == "" {
		return ""
	}

	// 解析 assignees JSON
	var assignees []struct {
		Login string `json:"login"`
	}
	if err := json.Unmarshal([]byte(assigneesJSON), &assignees); err != nil {
		// 如果解析失败，尝试解析单个对象
		var assignee struct {
			Login string `json:"login"`
		}
		if err2 := json.Unmarshal([]byte(assigneesJSON), &assignee); err2 != nil {
			return ""
		}
		assignees = []struct {
			Login string `json:"login"`
		}{assignee}
	}

	// 提取所有 login
	loginSet := make(map[string]bool)
	for _, assignee := range assignees {
		if assignee.Login != "" {
			loginSet[assignee.Login] = true
		}
	}

	// 解析用户映射
	var result strings.Builder
	pairs := strings.Split(userMaps, ",")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 {
			continue
		}

		username := strings.TrimSpace(parts[0])
		openID := strings.TrimSpace(parts[1])

		// 检查是否需要 @这个用户
		if loginSet[username] {
			// 飞书 @用户格式：<at user_id="ou_xxxx">用户名</at>
			result.WriteString(fmt.Sprintf(`<at user_id="%s">%s</at> `, openID, username))
		}
	}

	return result.String()
}
