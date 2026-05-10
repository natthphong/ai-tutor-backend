package api

import (
	"fmt"
	"github.com/gofiber/fiber/v2"
)

type Response struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Body    interface{} `json:"body,omitempty"`
}

func SuccessResponse(body interface{}) Response {
	return Response{
		Code:    "OK",
		Message: "Success",
		Body:    body,
	}
}

func Err(code string, message string) Response {
	return Response{
		Code:    code,
		Message: message,
	}
}

func ErrWithBody(code string, message string, body interface{}) Response {
	return Response{
		Code:    code,
		Message: message,
		Body:    body,
	}
}

func Ok(c *fiber.Ctx, body interface{}) error {
	return c.Status(fiber.StatusOK).JSON(SuccessResponse(body))
}

func OkWithMessage(c *fiber.Ctx, message string, body interface{}) error {
	return c.Status(fiber.StatusOK).JSON(Response{
		Code:    "OK",
		Message: message,
		Body:    body,
	})
}

func BadRequest(c *fiber.Ctx, message string) error {
	msg := "Bad Request"
	if message != "" {
		msg = message
	}
	return c.Status(fiber.StatusBadRequest).JSON(Err("BAD_REQUEST", msg))
}

func Unauthorized(c *fiber.Ctx) error {
	return c.Status(fiber.StatusUnauthorized).JSON(Err("UNAUTHORIZED", "Unauthorized"))
}

func Forbidden(c *fiber.Ctx) error {
	return c.Status(fiber.StatusForbidden).JSON(Err("FORBIDDEN", "Forbidden"))
}

func NotFound(c *fiber.Ctx, message string) error {
	msg := "Not Found"
	if message != "" {
		msg = message
	}
	return c.Status(fiber.StatusNotFound).JSON(Err("NOT_FOUND", msg))
}

func InternalError(c *fiber.Ctx, message string) error {
	return c.Status(fiber.StatusInternalServerError).JSON(Err("INTERNAL_ERROR", fmt.Sprintf("%s", message)))
}

func AIProviderError(c *fiber.Ctx, provider string, fallbackTried bool) error {
	return c.Status(fiber.StatusServiceUnavailable).JSON(ErrWithBody(
		"AI_PROVIDER_ERROR",
		"AI provider failed",
		fiber.Map{
			"provider":      provider,
			"fallbackTried": fallbackTried,
		},
	))
}

func SessionNotFound(c *fiber.Ctx) error {
	return c.Status(fiber.StatusNotFound).JSON(Err("SESSION_NOT_FOUND", "Session not found"))
}

func UnitNotFound(c *fiber.Ctx) error {
	return c.Status(fiber.StatusNotFound).JSON(Err("UNIT_NOT_FOUND", "Unit not found"))
}

func JwtError(c *fiber.Ctx, message string) error {
	return c.Status(fiber.StatusUnauthorized).JSON(Err("401", message))
}
