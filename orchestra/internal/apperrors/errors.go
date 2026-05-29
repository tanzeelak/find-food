package apperrors

type Error struct {
	Status int
	Code   string
	Detail string
}

func (e *Error) Error() string {
	return e.Detail
}

func New(status int, code string, detail string) *Error {
	return &Error{Status: status, Code: code, Detail: detail}
}
