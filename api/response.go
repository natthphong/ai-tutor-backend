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
		Code:    "000",
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
func JwtError(c *fiber.Ctx, message string) error {
	//if message == "Token Not Found" {
	//	return c.Status(fiber.StatusUnauthorized).JSON(Err("452", message))
	//} else if message == "Logout Already" {
	//	return c.Status(fiber.StatusUnauthorized).JSON(Err("453", message))
	//} else if message == "Token Expired" {
	//	return c.Status(fiber.StatusUnauthorized).JSON(Err("454", message))
	//} else if message == "Line VerifyIDToken Failed" {
	//	return c.Status(fiber.StatusUnauthorized).JSON(Err("455", message))
	//} else if message == "ERROR DB" {
	//	return c.Status(fiber.StatusInternalServerError).JSON(Err("500", message))
	//} else {
	//	return c.Status(fiber.StatusUnauthorized).JSON(Err("401", message))
	//}
	return c.Status(fiber.StatusUnauthorized).JSON(Err("401", message))

}

func Unauthorized(c *fiber.Ctx) error {
	return c.Status(fiber.StatusUnauthorized).JSON(Err("401", fiber.ErrUnauthorized.Error()))
}

func Forbidden(c *fiber.Ctx) error {
	return c.Status(fiber.StatusForbidden).JSON(Err("403", fiber.ErrForbidden.Error()))
}

func InternalError(c *fiber.Ctx, message string) error {
	return c.Status(fiber.StatusInternalServerError).JSON(Err("500", fmt.Sprintf("%s", message)))
}

func Ok(c *fiber.Ctx, body interface{}) error {
	return c.Status(fiber.StatusOK).JSON(SuccessResponse(body))
}

func BadRequest(c *fiber.Ctx, message string) error {
	msg := fiber.ErrBadRequest.Message
	if message != "" {
		msg = message
	}
	return c.Status(fiber.StatusBadRequest).JSON(Err("400", msg))
}
