package runtime

import (
	"encoding/xml"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
)

const reminderTimezone = "Etc/GMT-8"

var reminderLocation = time.FixedZone(reminderTimezone, 8*60*60)

var chineseWeekdays = [...]string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}

func buildInputMessages(query, skillCatalog string, now time.Time) ([]*schema.Message, error) {
	interactionRequest, err := xml.MarshalIndent(struct {
		XMLName   xml.Name `xml:"interaction_request"`
		UserQuery string   `xml:"user_query"`
	}{UserQuery: query}, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal interaction request: %w", err)
	}

	reminderParts := []string{currentDateHint(now)}
	if catalog := strings.TrimSpace(skillCatalog); catalog != "" {
		reminderParts = append(reminderParts, catalog)
	}
	reminder := "<system-reminder>\n" + strings.Join(reminderParts, "\n\n") + "\n</system-reminder>"

	return []*schema.Message{
		schema.UserMessage(reminder),
		schema.UserMessage(string(interactionRequest)),
	}, nil
}

func currentDateHint(now time.Time) string {
	localNow := now.In(reminderLocation)
	return fmt.Sprintf("当前日期：%s %s %s（%s）",
		localNow.Format("2006-01-02"),
		chineseWeekdays[localNow.Weekday()],
		localNow.Format("15:04:05"),
		reminderTimezone,
	)
}
