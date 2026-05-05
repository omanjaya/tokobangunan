package middleware

import (
	"log/slog"
	"time"

	"github.com/labstack/echo/v4"
)

// RequestLogger returns an Echo middleware that emits a single structured log
// line per request using the provided slog.Logger.
func RequestLogger(logger *slog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)

			req := c.Request()
			res := c.Response()

			status := res.Status
			if err != nil {
				// Echo may not have written the status yet; resolve via HTTPError.
				if he, ok := err.(*echo.HTTPError); ok {
					status = he.Code
				}
			}

			attrs := []any{
				slog.String("method", req.Method),
				slog.String("path", req.URL.Path),
				slog.Int("status", status),
				slog.Duration("latency", time.Since(start)),
				slog.String("request_id", res.Header().Get(echo.HeaderXRequestID)),
				slog.String("remote_ip", c.RealIP()),
			}

			switch {
			case status >= 500:
				if err != nil {
					attrs = append(attrs, slog.String("error", err.Error()))
				}
				logger.Error("request", attrs...)
			case status >= 400:
				logger.Warn("request", attrs...)
			default:
				logger.Info("request", attrs...)
			}
			return err
		}
	}
}
