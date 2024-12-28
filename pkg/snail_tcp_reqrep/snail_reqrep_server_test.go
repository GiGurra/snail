package snail_tcp_reqrep

import (
	"github.com/GiGurra/snail/pkg/snail_logging"
	"log/slog"
	"testing"
)

func TestNewServer_SendAndRespondWithJson(t *testing.T) {
	snail_logging.ConfigureDefaultLogger("text", "info", false)

	slog.Info("TestNewServer_SendAndRespondWithJson")
}
