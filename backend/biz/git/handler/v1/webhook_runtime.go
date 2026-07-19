package v1

import (
	"fmt"
	"strings"

	"github.com/Y-vQv-Y/DevLoom/backend/domain"
)

func webhookRuntime(bot *domain.GitBot) (string, error) {
	if bot == nil {
		return "", fmt.Errorf("git bot has no development host")
	}
	hostID := strings.TrimSpace(bot.HostID)
	if hostID == "" && bot.Host != nil {
		hostID = strings.TrimSpace(bot.Host.ID)
	}
	if hostID == "" {
		return "", fmt.Errorf("git bot has no development host")
	}

	return hostID, nil
}
