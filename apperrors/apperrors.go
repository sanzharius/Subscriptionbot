package apperrors

import "fmt"

type AppError struct {
	Message string
	Code    string
}

var (
	MessageUnmarshallingError = AppError{
		Message: "couldn't unmarshal a response",
		Code:    "UNMARSHAL_ERR",
	}
	ConfigReadErr = AppError{
		Message: "couldn't read config",
		Code:    "CONFIG_READ_ER",
	}
	DataNotFoundErr = AppError{
		Message: "cannot get a weather forecast",
		Code:    "DATA_NOT_FOUND_ERR",
	}
	APICallingErr = AppError{
		Code: "API_CALLING_ERR",
	}
	MongoDBDataNotFoundErr = AppError{
		Message: "data not found in mongodb",
		Code:    "MONGO_DB_DATA_NOT_FOUND_ERR",
	}
	MongoDBFindErr = AppError{
		Message: "could not find data in mongodb",
		Code:    "MONGO_DB_FIND_ERR",
	}
	MongoDBFindOneErr = AppError{
		Message: "could not find subscription in mongodb",
		Code:    "MONGO_DB_FIND_ERR",
	}
	MongoDBCursorErr = AppError{
		Message: "Got cursor error in mongodb",
		Code:    "MONGO_DB_CURSOR_ERR",
	}
	MongoDBUpdateErr = AppError{
		Message: "Could not update subscription",
		Code:    "MONGO_DB_UPDATE_ERR",
	}
	MongoDBDeleteErr = AppError{
		Message: "Could not delete subscription",
		Code:    "MONGO_DB_DELETE_ERR",
	}
	TimeParseErr = AppError{
		Message: "Could not parse time",
		Code:    "TIME_PARSE_ERR",
	}
)

func (appError *AppError) Error() string {
	return appError.Code + ": " + appError.Message
}
func (appError *AppError) AppendMessage(anyErrs ...interface{}) *AppError {
	return &AppError{
		Message: fmt.Sprintf("%v: %v", appError.Message, anyErrs),
		Code:    appError.Code,
	}
}
