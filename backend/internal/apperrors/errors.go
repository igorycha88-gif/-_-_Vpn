package apperrors

import "errors"

var (
	ErrNotFound     = errors.New("ресурс не найден")
	ErrConflict     = errors.New("конфликт данных")
	ErrValidation   = errors.New("ошибка валидации")
	ErrUnauthorized = errors.New("не авторизован")
)
