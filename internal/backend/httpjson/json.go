package httpjson

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/anhang204/Netcentric_Project/internal/contract"
	"github.com/gin-gonic/gin"
)

func Read(c *gin.Context, value any) error {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16*1024)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return errors.New("expected one JSON value")
	}
	return nil
}

func Error(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, contract.ErrorResponse{
		Error: contract.ErrorDetail{Code: code, Message: message},
	})
}
