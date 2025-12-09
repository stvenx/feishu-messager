#!/bin/bash
set -e

if [ -z "$BOT_TOKEN" ]; then
  echo "Please set the BOT_TOKEN secret."
  exit 1
fi

if [ -z "$POST_MESSAGE" ] && [ -z "$MESSAGE_FILE" ]; then
  echo "Please set the post message or a file containing the message."
  exit 1
fi

# 解析用户映射，生成 @用户的标签
# 格式：stvenx:ou_xxxx,user1:ou_yyyy
parse_users() {
    local mapstr=$1
    local touser=$2
    local result=""

    if [ -z "$mapstr" ] || [ -z "$touser" ]; then
        echo ""
        return
    fi

    IFS=',' read -ra pairs <<< "$mapstr"
    for pair in "${pairs[@]}"; do
        IFS=':' read -ra data <<< "$pair"
        key=${data[0]}
        value=${data[1]}
        if [[ " $touser " = *" $key "* ]]; then
            # 飞书 @用户格式：<at user_id="ou_xxxx">用户名</at>
            result="${result}<at user_id=\"${value}\">${key}</at> "
        fi
    done

    echo "$result"
}

# 处理 ASSIGNEES（如果存在），生成 @用户的标签
at_users=""
if [ -n "$ASSIGNEES" ] && [ -n "$USER_MAPS" ]; then
    ASSIGNEES_LOGIN=$(echo "$ASSIGNEES" | jq -r '.[] | .login' 2>/dev/null || echo "")
    if [ -n "$ASSIGNEES_LOGIN" ]; then
        ASSIGNEES_LOGIN="${ASSIGNEES_LOGIN//$'\n'/ }"
        at_users=$(parse_users "$USER_MAPS" "$ASSIGNEES_LOGIN")
    fi
fi

if [ -z "$POST_MESSAGE" ] && [ -n "$MESSAGE_FILE" ]; then
  if [ ! -f "$MESSAGE_FILE" ]; then
    echo "File '$MESSAGE_FILE' not found."
    exit 1
  fi
  POST_MESSAGE=$(cat "$MESSAGE_FILE")
fi

# 如果有需要 @的用户，在消息前添加（在转义之前）
if [ -n "$at_users" ]; then
    POST_MESSAGE="${at_users}${POST_MESSAGE}"
fi

# 转义消息内容（转义换行和双引号）
POST_MESSAGE=$(echo "$POST_MESSAGE" | sed -e ':a' -e 'N' -e '$!ba' -e 's/\n/\\n/g' -e 's/"/\\"/g')

# 飞书机器人的 webhook URL
WEBHOOK_URL="https://open.feishu.cn/open-apis/bot/v2/hook/${BOT_TOKEN}"

if [ "$MSG_TYPE" = "text" ]; then
  curl -vv -X POST "${WEBHOOK_URL}" \
    -H "Content-Type: application/json" \
    -d @- <<END
{
  "msg_type": "text",
  "content": {
    "text": "${POST_MESSAGE}"
  }
}
END
elif [ "$MSG_TYPE" = "markdown" ]; then
  # 飞书 text 类型支持基本的 markdown 语法（如 **粗体**、*斜体* 等）
  # 如果需要更复杂的富文本格式，可以使用 post 或 interactive card 类型
  curl -vv -X POST "${WEBHOOK_URL}" \
    -H "Content-Type: application/json" \
    -d @- <<END
{
  "msg_type": "text",
  "content": {
    "text": "${POST_MESSAGE}"
  }
}
END
else
  echo "Unsupported MSG_TYPE: ${MSG_TYPE}. Supported types: text, markdown"
  exit 1
fi

