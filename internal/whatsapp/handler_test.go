package whatsapp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestSendMessageHandler(t *testing.T) {
	e := echo.New()
	h, _ := New()
	a := assert.New(t)

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(
		`{"to":"6598232744","`,
	))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if a.NoError(h.SendMessage(c)) {
		a.Equal(http.StatusOK, rec.Code)
	}
}
