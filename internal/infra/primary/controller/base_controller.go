package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"io"
	"jwt/base/internal/infra/config"
	"jwt/base/internal/infra/primary/controller/dto"
	"net/http"
	"strconv"
)

func handleBind(t config.ITranslate, c *gin.Context, data interface{}) bool {
	if err := c.ShouldBindJSON(&data); err != nil {
		if err == io.EOF {
			c.JSON(http.StatusBadRequest, dto.ResponseDto{
				Status:  http.StatusBadRequest,
				Message: t.TranslateByLang(c.GetHeader("lang"), "errors.no-data-form", nil),
				Data:    nil,
			})
			return false
		}

		var fieldErrors []string
		for _, fieldErr := range err.(validator.ValidationErrors) {
			if fieldErr.Field() == "Email" && fieldErr.Tag() == "email" {
				fieldErrors = append(fieldErrors, t.TranslateByLang(c.GetHeader("lang"), "errors.invalid-email", nil))
			} else {
				fieldErrors = append(fieldErrors, t.TranslateByLang(c.GetHeader("lang"), "errors.invalid-field-required", map[string]interface{}{"Field": fieldErr.Field()}))
			}
		}
		c.JSON(http.StatusBadRequest, dto.ResponseDto{
			Status:  http.StatusBadRequest,
			Message: "Error",
			Data:    fieldErrors,
		})
		return false
	}
	return true
}

func handlePagination(c *gin.Context) (int, int, int) {
	page := c.DefaultQuery("page", "1")
	pageSize := c.DefaultQuery("pageSize", "10")
	pageInt, _ := strconv.Atoi(page)
	pageSizeInt, _ := strconv.Atoi(pageSize)

	if pageInt < 1 {
		pageInt = 1
	}
	if pageSizeInt < 1 {
		pageSizeInt = 10
	}

	offset := (pageInt - 1) * pageSizeInt
	return pageInt, pageSizeInt, offset
}
