package slack_metric_exporter

import (
	"fmt"
	"log"

	"github.com/slack-go/slack"
)

func SendSlackMessage(token, channelID, message string) error {
	api := slack.New(token)

	log.Println(token, channelID)

	// PostMessage의 첫 번째 인자는 채널 ID, 두 번째 이후는 메시지 옵션입니다.
	_, _, err := api.PostMessage(
		channelID,
		slack.MsgOptionText(message, false),
	)
	if err != nil {
		return fmt.Errorf("failed to post message: %w", err)
	}
	return nil
}
