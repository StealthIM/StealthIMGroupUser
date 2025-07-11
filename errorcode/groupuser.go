package errorcode

const (
	// GroupUserInternalError 一般内部错误
	GroupUserInternalError int32 = 1300 + iota
	// GroupUserDatabaseError 数据库操作错误
	GroupUserDatabaseError
	// GroupUserNotFound 群组或用户未找到
	GroupUserNotFound
	// GroupUserPermissionDenied 权限不足
	GroupUserPermissionDenied
	// GroupUserAlreadyInGroup 用户已在群组中
	GroupUserAlreadyInGroup
	// GroupUserPasswordIncorrect 密码不正确
	GroupUserPasswordIncorrect
	// GroupUserQueryError 用户查询错误
	GroupUserQueryError
	// GroupUserInsertError 插入操作错误
	GroupUserInsertError
)
