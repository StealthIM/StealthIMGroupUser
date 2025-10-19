package grpc

import (
	"context"
	"encoding/hex"
	"fmt"

	pb_gtw "StealthIMGroupUser/StealthIM.DBGateway"
	pb "StealthIMGroupUser/StealthIM.GroupUser"
	"StealthIMGroupUser/config"
	"StealthIMGroupUser/errorcode"
	"StealthIMGroupUser/gateway"
	"StealthIMGroupUser/user"
	"crypto/sha256"

	"google.golang.org/protobuf/proto"
)

// GetGroupsByUID 获取用户加入的群组列表
func (s *server) GetGroupsByUID(ctx context.Context, req *pb.GetGroupsByUIDRequest) (*pb.GetGroupsByUIDResponse, error) {

	resp, err := gateway.ExecRedisBGet(&pb_gtw.RedisGetBytesRequest{DBID: 0, Key: "groupuser:groups:" + fmt.Sprintf("%d", req.Uid)})

	cacheObj := &pb.GetGroupsByUIDCache{}

	if err != nil || resp.Result.Code != errorcode.Success || len(resp.Value) == 0 || (proto.Unmarshal(resp.Value, cacheObj) != nil) {
		username, err := user.QueryUsernameByUID(ctx, req.Uid)
		if err != nil {
			return &pb.GetGroupsByUIDResponse{
				Result: &pb.Result{Code: errorcode.GroupUserQueryError, Msg: fmt.Sprintf("User query error: %v", err)},
			}, nil
		}
		// 查询group_user_table获取用户群组
		sqlReq := &pb_gtw.SqlRequest{
			Sql: "SELECT groupid FROM `group_user_table` WHERE username = ?",
			Db:  pb_gtw.SqlDatabases_Groups,
			Params: []*pb_gtw.InterFaceType{
				{Response: &pb_gtw.InterFaceType_Str{Str: username}},
			},
		}

		sqlResp, err := gateway.ExecSQL(sqlReq)
		if err != nil {
			return &pb.GetGroupsByUIDResponse{
				Result: &pb.Result{Code: errorcode.GroupUserDatabaseError, Msg: fmt.Sprintf("Database error: %v", err)},
			}, nil
		}
		if sqlResp.Result.Code != errorcode.Success {
			return &pb.GetGroupsByUIDResponse{
				Result: &pb.Result{Code: sqlResp.Result.Code, Msg: sqlResp.Result.Msg},
			}, nil
		}

		// 解析群组ID列表
		var groups []int32
		for _, row := range sqlResp.Data {
			if len(row.Result) > 0 {
				groupID, ok := row.Result[0].Response.(*pb_gtw.InterFaceType_Int32)
				if ok {
					groups = append(groups, groupID.Int32)
				}
			}
		}

		cacheObj.Groups = groups

		// 将结果缓存到Redis
		cacheBytes, err := proto.Marshal(cacheObj)
		if err == nil {
			go gateway.ExecRedisBSet(&pb_gtw.RedisSetBytesRequest{DBID: 0, Key: "groupuser:groups:" + fmt.Sprintf("%d", req.Uid), Value: cacheBytes})
		}
	}
	return &pb.GetGroupsByUIDResponse{
		Result: &pb.Result{Code: errorcode.Success},
		Groups: cacheObj.Groups,
	}, nil
}

// GetGroupPublicInfo 获取群组公开信息
func (s *server) GetGroupPublicInfo(ctx context.Context, req *pb.GetGroupPublicInfoRequest) (*pb.GetGroupPublicInfoResponse, error) {
	resp, err := gateway.ExecRedisBGet(&pb_gtw.RedisGetBytesRequest{DBID: 0, Key: "groupuser:public:" + fmt.Sprintf("%d", req.GroupId)})
	cacheObj := &pb.GetGroupPublicInfoCache{}
	if err != nil || resp.Result.Code != errorcode.Success || len(resp.Value) == 0 || (proto.Unmarshal(resp.Value, cacheObj) != nil) {
		for { // 利用控制流跳出
			sqlReq := &pb_gtw.SqlRequest{
				Sql: "SELECT `name`, `create_time` FROM `groups` WHERE groupid = ?",
				Db:  pb_gtw.SqlDatabases_Groups,
				Params: []*pb_gtw.InterFaceType{
					{Response: &pb_gtw.InterFaceType_Int32{Int32: req.GroupId}},
				},
			}

			sqlResp, err := gateway.ExecSQL(sqlReq)
			if err != nil {
				return &pb.GetGroupPublicInfoResponse{
					Result: &pb.Result{Code: errorcode.GroupUserDatabaseError, Msg: fmt.Sprintf("Database error: %v", err)},
				}, nil
			}
			if sqlResp.Result.Code != errorcode.Success || len(sqlResp.Data) == 0 {
				cacheObj.Id = -1
				break
			}

			row := sqlResp.Data[0]
			cacheObj.Id = req.GroupId
			cacheObj.Name = row.Result[0].GetStr()
			cacheObj.CreatedAt = row.Result[1].GetInt64()
			if true { // 保证始终跳出
				break
			}
		}
		cacheBytes, err := proto.Marshal(cacheObj)
		if err == nil {
			go gateway.ExecRedisBSet(&pb_gtw.RedisSetBytesRequest{DBID: 0, Key: "groupuser:public:" + fmt.Sprintf("%d", req.GroupId), Value: cacheBytes})
		}
	}
	if cacheObj.Id == -1 {
		return &pb.GetGroupPublicInfoResponse{
			Result: &pb.Result{Code: errorcode.GroupUserNotFound, Msg: "Group not found"},
		}, nil
	}
	return &pb.GetGroupPublicInfoResponse{
		Result:    &pb.Result{Code: errorcode.Success},
		Id:        cacheObj.Id,
		Name:      cacheObj.Name,
		CreatedAt: cacheObj.CreatedAt,
	}, nil
}

func convertSQLUserTypeToProto(sqlUserType string) pb.MemberType {
	switch sqlUserType {
	case "member":
		return pb.MemberType_member
	case "manager":
		return pb.MemberType_manager
	case "owner":
		return pb.MemberType_owner
	case "other":
		return pb.MemberType_other
	default:
		return pb.MemberType_other
	}
}
func convertProtoToSQLUserType(protoType pb.MemberType) string {
	switch protoType {
	case pb.MemberType_member:
		return "member"
	case pb.MemberType_manager:
		return "manager"
	case pb.MemberType_owner:
		return "owner"
	case pb.MemberType_other:
		return "other"
	default:
		return "other"
	}
}

// GetGroupInfo 获取群组信息
func (s *server) GetGroupInfo(ctx context.Context, req *pb.GetGroupInfoRequest) (*pb.GetGroupInfoResponse, error) {
	resp, err := gateway.ExecRedisBGet(&pb_gtw.RedisGetBytesRequest{DBID: 0, Key: "groupuser:info:" + fmt.Sprintf("%d", req.GroupId)})
	cacheObj := &pb.GetGroupInfoCache{}
	if err != nil || resp.Result.Code != errorcode.Success || len(resp.Value) == 0 || (proto.Unmarshal(resp.Value, cacheObj) != nil) {
		for { // 利用控制流跳出
			sqlReq := &pb_gtw.SqlRequest{
				Sql: "SELECT `username`, CAST(`type` AS CHAR) FROM `group_user_table` WHERE groupid = ?",
				Db:  pb_gtw.SqlDatabases_Groups,
				Params: []*pb_gtw.InterFaceType{
					{Response: &pb_gtw.InterFaceType_Int32{Int32: req.GroupId}},
				},
			}

			sqlResp, err := gateway.ExecSQL(sqlReq)
			if err != nil {
				return &pb.GetGroupInfoResponse{
					Result: &pb.Result{Code: errorcode.GroupUserDatabaseError, Msg: fmt.Sprintf("Database error: %v", err)},
				}, nil
			}
			if sqlResp.Result.Code != errorcode.Success || len(sqlResp.Data) == 0 {
				break
			}

			for _, row := range sqlResp.Data {
				cacheObj.Members = append(cacheObj.Members, &pb.MemberObject{
					Name: row.Result[0].GetStr(),
					Type: convertSQLUserTypeToProto(row.Result[1].GetStr()),
				})
			}
			if true { // 保证始终跳出
				break
			}
		}
		cacheBytes, err := proto.Marshal(cacheObj)
		if err == nil {
			go gateway.ExecRedisBSet(&pb_gtw.RedisSetBytesRequest{DBID: 0, Key: "groupuser:info:" + fmt.Sprintf("%d", req.GroupId), Value: cacheBytes})
		}
	}
	if len(cacheObj.Members) == 0 {
		return &pb.GetGroupInfoResponse{
			Result: &pb.Result{Code: errorcode.GroupUserNotFound, Msg: "Group not found"},
		}, nil
	}

	// 添加用户到群组
	username, err := user.QueryUsernameByUID(ctx, req.Uid)
	if err != nil {
		return &pb.GetGroupInfoResponse{
			Result: &pb.Result{Code: errorcode.GroupUserQueryError, Msg: fmt.Sprintf("User query error: %v", err)},
		}, nil
	}

	found := false
	for _, element := range cacheObj.Members {
		if element.Name == username {
			found = true
			break
		}
	}
	if !found {
		return &pb.GetGroupInfoResponse{
			Result: &pb.Result{Code: errorcode.GroupUserPermissionDenied, Msg: "Permission denied"},
		}, nil
	}
	return &pb.GetGroupInfoResponse{
		Result:  &pb.Result{Code: errorcode.Success},
		Members: cacheObj.Members,
	}, nil
}

// JoinGroup 用户加入群组
func (s *server) JoinGroup(ctx context.Context, req *pb.JoinGroupRequest) (*pb.JoinGroupResponse, error) {
	// 验证群组密码
	resp, err := gateway.ExecRedisGet(&pb_gtw.RedisGetStringRequest{DBID: 0, Key: "groupuser:password:" + fmt.Sprintf("%d", req.GroupId)})
	storedPasswordHash := ""
	if err != nil || resp.Result.Code != errorcode.Success || len(resp.Value) == 0 || resp.Value != req.Password {
		sqlReq := &pb_gtw.SqlRequest{
			Sql: "SELECT `password` FROM `groups` WHERE groupid = ?",
			Db:  pb_gtw.SqlDatabases_Groups,
			Params: []*pb_gtw.InterFaceType{
				{Response: &pb_gtw.InterFaceType_Int32{Int32: req.GroupId}},
			},
		}

		sqlResp, err := gateway.ExecSQL(sqlReq)
		if err != nil {
			return &pb.JoinGroupResponse{
				Result: &pb.Result{Code: errorcode.GroupUserDatabaseError, Msg: fmt.Sprintf("Database error: %v", err)},
			}, nil
		}
		if sqlResp.Result.Code != errorcode.Success || len(sqlResp.Data) == 0 {
			return &pb.JoinGroupResponse{
				Result: &pb.Result{Code: errorcode.GroupUserNotFound, Msg: "Group not found"},
			}, nil
		}

		storedPasswordHash = sqlResp.Data[0].Result[0].GetStr()
		if storedPasswordHash == "" {
			return &pb.JoinGroupResponse{
				Result: &pb.Result{Code: errorcode.GroupUserNotFound, Msg: "Group not found"},
			}, nil
		}
		go gateway.ExecRedisSet(&pb_gtw.RedisSetStringRequest{DBID: 0, Key: "groupuser:password:" + fmt.Sprintf("%d", req.GroupId), Value: storedPasswordHash})
	} else {
		storedPasswordHash = resp.Value
	}

	if storedPasswordHash == "" {
		return &pb.JoinGroupResponse{
			Result: &pb.Result{Code: errorcode.GroupUserNotFound, Msg: "Group not found"},
		}, nil
	}

	hashedPasswordRequest := sha256.Sum256([]byte(req.Password + config.LatestConfig.Security.PasswordSalt))
	if hex.EncodeToString(hashedPasswordRequest[:]) != storedPasswordHash {
		return &pb.JoinGroupResponse{
			Result: &pb.Result{Code: errorcode.GroupUserPasswordIncorrect, Msg: "Password incorrect"},
		}, nil
	}

	// 添加用户到群组
	username, err := user.QueryUsernameByUID(ctx, req.Uid)
	if err != nil {
		return &pb.JoinGroupResponse{
			Result: &pb.Result{Code: errorcode.GroupUserQueryError, Msg: fmt.Sprintf("User query error: %v", err)},
		}, nil
	}

	respCache, err := gateway.ExecRedisBGet(&pb_gtw.RedisGetBytesRequest{DBID: 0, Key: "groupuser:info:" + fmt.Sprintf("%d", req.GroupId)})
	cacheObj := &pb.GetGroupInfoCache{}
	if !(err != nil || resp.Result.Code != errorcode.Success || len(resp.Value) == 0 || (proto.Unmarshal(respCache.Value, cacheObj) != nil)) {
		for _, element := range cacheObj.Members {
			if element.Name == username {
				return &pb.JoinGroupResponse{
					Result: &pb.Result{Code: errorcode.GroupUserAlreadyInGroup, Msg: "User already in group"},
				}, nil
			}
		}
	}

	insertReq := &pb_gtw.SqlRequest{
		Sql:    "INSERT INTO group_user_table (groupid, username, type) VALUES (?, ?, 'member')",
		Db:     pb_gtw.SqlDatabases_Groups,
		Commit: true,
		Params: []*pb_gtw.InterFaceType{
			{Response: &pb_gtw.InterFaceType_Int32{Int32: req.GroupId}},
			{Response: &pb_gtw.InterFaceType_Str{Str: username}},
		},
		GetRowCount: true,
	}

	insertResp, err := gateway.ExecSQL(insertReq)

	if err != nil {
		return &pb.JoinGroupResponse{
			Result: &pb.Result{Code: errorcode.GroupUserDatabaseError, Msg: fmt.Sprintf("Insert error: %v", err)}}, nil
	}

	if insertResp.Result.Code != errorcode.Success {
		return &pb.JoinGroupResponse{
			Result: &pb.Result{Code: insertResp.Result.Code, Msg: insertResp.Result.Msg},
		}, nil
	}

	if insertResp.RowsAffected == 0 {
		return &pb.JoinGroupResponse{
			Result: &pb.Result{Code: errorcode.GroupUserAlreadyInGroup, Msg: "User already in group"},
		}, nil
	}

	go gateway.ExecRedisDel(&pb_gtw.RedisDelRequest{DBID: 0, Key: "groupuser:groups:" + fmt.Sprintf("%d", req.Uid)})
	go gateway.ExecRedisDel(&pb_gtw.RedisDelRequest{DBID: 0, Key: "groupuser:info:" + fmt.Sprintf("%d", req.GroupId)})
	return &pb.JoinGroupResponse{
		Result: &pb.Result{Code: errorcode.Success, Msg: ""},
	}, nil
}

// InviteGroup 用户被拉入群组
func (s *server) InviteGroup(ctx context.Context, req *pb.InviteGroupRequest) (*pb.InviteGroupResponse, error) {
	username, err := user.QueryUsernameByUID(ctx, req.Uid)
	if err != nil {
		return &pb.InviteGroupResponse{
			Result: &pb.Result{Code: errorcode.GroupUserQueryError, Msg: fmt.Sprintf("User query error: %v", err)},
		}, nil
	}

	if !user.QueryHasUsername(ctx, req.Username) {
		return &pb.InviteGroupResponse{
			Result: &pb.Result{Code: errorcode.GroupUserNotFound, Msg: "User not found"},
		}, nil
	}

	resp, err := gateway.ExecRedisBGet(&pb_gtw.RedisGetBytesRequest{DBID: 0, Key: "groupuser:info:" + fmt.Sprintf("%d", req.GroupId)})
	cacheObj := &pb.GetGroupInfoCache{}
	if err != nil || resp.Result.Code != errorcode.Success || len(resp.Value) == 0 || (proto.Unmarshal(resp.Value, cacheObj) != nil) {
		for { // 利用控制流跳出
			sqlReq := &pb_gtw.SqlRequest{
				Sql: "SELECT `username`, CAST(`type` AS CHAR) FROM `group_user_table` WHERE groupid = ?",
				Db:  pb_gtw.SqlDatabases_Groups,
				Params: []*pb_gtw.InterFaceType{
					{Response: &pb_gtw.InterFaceType_Int32{Int32: req.GroupId}},
				},
			}

			sqlResp, err := gateway.ExecSQL(sqlReq)
			if err != nil {
				return &pb.InviteGroupResponse{
					Result: &pb.Result{Code: errorcode.GroupUserDatabaseError, Msg: fmt.Sprintf("Database error: %v", err)},
				}, nil
			}
			if sqlResp.Result.Code != errorcode.Success || len(sqlResp.Data) == 0 {
				break
			}

			for _, row := range sqlResp.Data {
				cacheObj.Members = append(cacheObj.Members, &pb.MemberObject{
					Name: row.Result[0].GetStr(),
					Type: convertSQLUserTypeToProto(row.Result[1].GetStr()),
				})
			}
			if true { // 保证始终跳出
				break
			}
		}
		cacheBytes, err := proto.Marshal(cacheObj)
		if err == nil {
			go gateway.ExecRedisBSet(&pb_gtw.RedisSetBytesRequest{DBID: 0, Key: "groupuser:info:" + fmt.Sprintf("%d", req.GroupId), Value: cacheBytes})
		}
	}
	if len(cacheObj.Members) == 0 {
		return &pb.InviteGroupResponse{
			Result: &pb.Result{Code: errorcode.GroupUserNotFound, Msg: "Group not found"},
		}, nil
	}
	found := false
	for _, element := range cacheObj.Members {
		switch element.Name {
		case username:
			found = true
		case req.Username:
			return &pb.InviteGroupResponse{
				Result: &pb.Result{Code: errorcode.GroupUserAlreadyInGroup, Msg: "User already in group"},
			}, nil
		}
	}
	if !found {
		return &pb.InviteGroupResponse{
			Result: &pb.Result{Code: errorcode.GroupUserPermissionDenied, Msg: "Permission denied"},
		}, nil
	}

	insertReq := &pb_gtw.SqlRequest{
		Sql:    "INSERT INTO group_user_table (`groupid`, `username`, `type`) VALUES (?, ?, 'member')",
		Db:     pb_gtw.SqlDatabases_Groups,
		Commit: true,
		Params: []*pb_gtw.InterFaceType{
			{Response: &pb_gtw.InterFaceType_Int32{Int32: req.GroupId}},
			{Response: &pb_gtw.InterFaceType_Str{Str: req.Username}},
		},
	}

	insertResp, err := gateway.ExecSQL(insertReq)

	if err != nil {
		return &pb.InviteGroupResponse{
			Result: &pb.Result{Code: errorcode.GroupUserDatabaseError, Msg: fmt.Sprintf("Insert error: %v", err)}}, nil
	}

	if insertResp.Result.Code != errorcode.Success {
		return &pb.InviteGroupResponse{
			Result: &pb.Result{Code: insertResp.Result.Code, Msg: insertResp.Result.Msg},
		}, nil
	}
	go func() {
		userID, err := user.QueryUIDByUsername(ctx, req.Username)
		if err != nil {
			return
		}
		gateway.ExecRedisDel(&pb_gtw.RedisDelRequest{DBID: 0, Key: "groupuser:groups:" + fmt.Sprintf("%d", userID)})
	}()
	go gateway.ExecRedisDel(&pb_gtw.RedisDelRequest{DBID: 0, Key: "groupuser:info:" + fmt.Sprintf("%d", req.GroupId)})
	return &pb.InviteGroupResponse{
		Result: &pb.Result{Code: errorcode.Success, Msg: ""},
	}, nil
}

// CreateGroup 创建群组
func (s *server) CreateGroup(ctx context.Context, req *pb.CreateGroupRequest) (*pb.CreateGroupResponse, error) {
	// 创建群组
	username, err := user.QueryUsernameByUID(ctx, req.Uid)
	if err != nil {
		return &pb.CreateGroupResponse{
			Result: &pb.Result{Code: errorcode.GroupUserQueryError, Msg: fmt.Sprintf("User query error: %v", err)},
		}, nil
	}
	hashedPasswordRequest := sha256.Sum256([]byte("" + config.LatestConfig.Security.PasswordSalt))
	insertReq := &pb_gtw.SqlRequest{
		Sql:    "INSERT INTO `groups` (`password`, `name`, `owner_uid`) VALUES (?, ?, ?)",
		Db:     pb_gtw.SqlDatabases_Groups,
		Commit: true,
		Params: []*pb_gtw.InterFaceType{
			{Response: &pb_gtw.InterFaceType_Str{Str: hex.EncodeToString(hashedPasswordRequest[:])}},
			{Response: &pb_gtw.InterFaceType_Str{Str: req.Name}},
			{Response: &pb_gtw.InterFaceType_Int32{Int32: req.Uid}},
		},
		GetLastInsertId: true,
	}

	insertResp, err := gateway.ExecSQL(insertReq)

	if err != nil {
		return &pb.CreateGroupResponse{
			Result: &pb.Result{Code: errorcode.GroupUserDatabaseError, Msg: fmt.Sprintf("Insert error: %v", err)},
		}, nil
	}

	if insertResp.Result.Code != errorcode.Success {
		return &pb.CreateGroupResponse{
			Result: &pb.Result{Code: insertResp.Result.Code, Msg: insertResp.Result.Msg},
		}, nil
	}

	insertReq2 := &pb_gtw.SqlRequest{
		Sql:    "INSERT INTO `group_user_table` (`groupid`, `username`, `type`) VALUES (?, ?, 'owner')",
		Db:     pb_gtw.SqlDatabases_Groups,
		Commit: true,
		Params: []*pb_gtw.InterFaceType{
			{Response: &pb_gtw.InterFaceType_Int32{Int32: int32(insertResp.LastInsertId)}},
			{Response: &pb_gtw.InterFaceType_Str{Str: username}},
		},
	}

	respInst, err := gateway.ExecSQL(insertReq2)
	if err != nil || respInst.Result.Code != errorcode.Success {
		go func() {
			gateway.ExecSQL(&pb_gtw.SqlRequest{
				Sql:    "DELETE FROM `groups` WHERE `id` = ?",
				Db:     pb_gtw.SqlDatabases_Groups,
				Commit: true,
				Params: []*pb_gtw.InterFaceType{
					{Response: &pb_gtw.InterFaceType_Int32{Int32: int32(insertResp.LastInsertId)}},
				},
			})
		}()
		return &pb.CreateGroupResponse{
			Result: &pb.Result{Code: errorcode.GroupUserDatabaseError, Msg: fmt.Sprintf("Insert error: %v", err)},
		}, nil
	}

	go gateway.ExecRedisDel(&pb_gtw.RedisDelRequest{DBID: 0, Key: "groupuser:groups:" + fmt.Sprintf("%d", req.Uid)})

	return &pb.CreateGroupResponse{
		Result:  &pb.Result{Code: errorcode.Success, Msg: ""},
		GroupId: int32(insertResp.LastInsertId),
	}, nil
}

// SetUserType 设置用户类型
func (s *server) SetUserType(ctx context.Context, req *pb.SetUserTypeRequest) (*pb.SetUserTypeResponse, error) {
	username, err := user.QueryUsernameByUID(ctx, req.Uid)
	if err != nil {
		return &pb.SetUserTypeResponse{
			Result: &pb.Result{Code: errorcode.GroupUserQueryError, Msg: fmt.Sprintf("User query error: %v", err)},
		}, nil
	}

	resp, err := gateway.ExecRedisBGet(&pb_gtw.RedisGetBytesRequest{DBID: 0, Key: "groupuser:info:" + fmt.Sprintf("%d", req.GroupId)})
	cacheObj := &pb.GetGroupInfoCache{}
	if err != nil || resp.Result.Code != errorcode.Success || len(resp.Value) == 0 || (proto.Unmarshal(resp.Value, cacheObj) != nil) {
		for { // 利用控制流跳出
			sqlReq := &pb_gtw.SqlRequest{
				Sql: "SELECT `username`, CAST(`type` AS CHAR) FROM `group_user_table` WHERE groupid = ?",
				Db:  pb_gtw.SqlDatabases_Groups,
				Params: []*pb_gtw.InterFaceType{
					{Response: &pb_gtw.InterFaceType_Int32{Int32: req.GroupId}},
				},
			}

			sqlResp, err := gateway.ExecSQL(sqlReq)
			if err != nil {
				return &pb.SetUserTypeResponse{
					Result: &pb.Result{Code: errorcode.GroupUserDatabaseError, Msg: fmt.Sprintf("Database error: %v", err)},
				}, nil
			}
			if sqlResp.Result.Code != errorcode.Success || len(sqlResp.Data) == 0 {
				break
			}

			for _, row := range sqlResp.Data {
				cacheObj.Members = append(cacheObj.Members, &pb.MemberObject{
					Name: row.Result[0].GetStr(),
					Type: convertSQLUserTypeToProto(row.Result[1].GetStr()),
				})
			}
			if true { // 保证始终跳出
				break
			}
		}
		cacheBytes, err := proto.Marshal(cacheObj)
		if err == nil {
			go gateway.ExecRedisBSet(&pb_gtw.RedisSetBytesRequest{DBID: 0, Key: "groupuser:info:" + fmt.Sprintf("%d", req.GroupId), Value: cacheBytes})
		}
	}
	if len(cacheObj.Members) == 0 {
		return &pb.SetUserTypeResponse{
			Result: &pb.Result{Code: errorcode.GroupUserNotFound, Msg: "Group not found"},
		}, nil
	}
	found := false
	foundDist := false
	for _, element := range cacheObj.Members {
		switch element.Name {
		case username:
			found = true
			if element.Type != pb.MemberType_owner {
				return &pb.SetUserTypeResponse{
					Result: &pb.Result{Code: errorcode.GroupUserPermissionDenied, Msg: "Permission denied"},
				}, nil
			}
		case req.Username:
			foundDist = true
		}
	}
	if !found {
		return &pb.SetUserTypeResponse{
			Result: &pb.Result{Code: errorcode.GroupUserPermissionDenied, Msg: "Permission denied"},
		}, nil
	}
	if !foundDist {
		return &pb.SetUserTypeResponse{
			Result: &pb.Result{Code: errorcode.GroupUserNotFound, Msg: "User not found"},
		}, nil
	}

	insertReq := &pb_gtw.SqlRequest{
		Sql:    "UPDATE `group_user_table` SET `type` = ? WHERE `groupid` = ? AND `username` = ?",
		Db:     pb_gtw.SqlDatabases_Groups,
		Commit: true,
		Params: []*pb_gtw.InterFaceType{
			{Response: &pb_gtw.InterFaceType_Str{Str: convertProtoToSQLUserType(req.Type)}},
			{Response: &pb_gtw.InterFaceType_Int32{Int32: req.GroupId}},
			{Response: &pb_gtw.InterFaceType_Str{Str: req.Username}},
		},
	}

	insertResp, err := gateway.ExecSQL(insertReq)

	if err != nil {
		return &pb.SetUserTypeResponse{
			Result: &pb.Result{Code: errorcode.GroupUserDatabaseError, Msg: fmt.Sprintf("Insert error: %v", err)}}, nil
	}

	if insertResp.Result.Code != errorcode.Success {
		return &pb.SetUserTypeResponse{
			Result: &pb.Result{Code: insertResp.Result.Code, Msg: insertResp.Result.Msg},
		}, nil
	}
	go gateway.ExecRedisDel(&pb_gtw.RedisDelRequest{DBID: 0, Key: "groupuser:info:" + fmt.Sprintf("%d", req.GroupId)})
	return &pb.SetUserTypeResponse{
		Result: &pb.Result{Code: errorcode.Success, Msg: ""},
	}, nil
}

// ChangeGroupName 设置群名
func (s *server) ChangeGroupName(ctx context.Context, req *pb.ChangeGroupNameRequest) (*pb.ChangeGroupNameResponse, error) {
	username, err := user.QueryUsernameByUID(ctx, req.Uid)
	if err != nil {
		return &pb.ChangeGroupNameResponse{
			Result: &pb.Result{Code: errorcode.GroupUserQueryError, Msg: fmt.Sprintf("User query error: %v", err)},
		}, nil
	}

	resp, err := gateway.ExecRedisBGet(&pb_gtw.RedisGetBytesRequest{DBID: 0, Key: "groupuser:info:" + fmt.Sprintf("%d", req.GroupId)})
	cacheObj := &pb.GetGroupInfoCache{}
	if err != nil || resp.Result.Code != errorcode.Success || len(resp.Value) == 0 || (proto.Unmarshal(resp.Value, cacheObj) != nil) {
		for { // 利用控制流跳出
			sqlReq := &pb_gtw.SqlRequest{
				Sql: "SELECT `username`, CAST(`type` AS CHAR) FROM `group_user_table` WHERE groupid = ?",
				Db:  pb_gtw.SqlDatabases_Groups,
				Params: []*pb_gtw.InterFaceType{
					{Response: &pb_gtw.InterFaceType_Int32{Int32: req.GroupId}},
				},
			}

			sqlResp, err := gateway.ExecSQL(sqlReq)
			if err != nil {
				return &pb.ChangeGroupNameResponse{
					Result: &pb.Result{Code: errorcode.GroupUserDatabaseError, Msg: fmt.Sprintf("Database error: %v", err)},
				}, nil
			}
			if sqlResp.Result.Code != errorcode.Success || len(sqlResp.Data) == 0 {
				break
			}

			for _, row := range sqlResp.Data {
				cacheObj.Members = append(cacheObj.Members, &pb.MemberObject{
					Name: row.Result[0].GetStr(),
					Type: convertSQLUserTypeToProto(row.Result[1].GetStr()),
				})
			}
			if true { // 保证始终跳出
				break
			}
		}
		cacheBytes, err := proto.Marshal(cacheObj)
		if err == nil {
			go gateway.ExecRedisBSet(&pb_gtw.RedisSetBytesRequest{DBID: 0, Key: "groupuser:info:" + fmt.Sprintf("%d", req.GroupId), Value: cacheBytes})
		}
	}
	if len(cacheObj.Members) == 0 {
		return &pb.ChangeGroupNameResponse{
			Result: &pb.Result{Code: errorcode.GroupUserNotFound, Msg: "Group not found"},
		}, nil
	}
	found := false
	for _, element := range cacheObj.Members {
		if element.Name == username {
			found = true
			if element.Type != pb.MemberType_owner && element.Type != pb.MemberType_manager {
				return &pb.ChangeGroupNameResponse{
					Result: &pb.Result{Code: errorcode.GroupUserPermissionDenied, Msg: "Permission denied"},
				}, nil
			}
		}
	}
	if !found {
		return &pb.ChangeGroupNameResponse{
			Result: &pb.Result{Code: errorcode.GroupUserPermissionDenied, Msg: "Permission denied"},
		}, nil
	}

	insertReq := &pb_gtw.SqlRequest{
		Sql:    "UPDATE `groups` SET `name` = ? WHERE `groupid` = ?",
		Db:     pb_gtw.SqlDatabases_Groups,
		Commit: true,
		Params: []*pb_gtw.InterFaceType{
			{Response: &pb_gtw.InterFaceType_Str{Str: req.Name}},
			{Response: &pb_gtw.InterFaceType_Int32{Int32: req.GroupId}},
		},
	}

	insertResp, err := gateway.ExecSQL(insertReq)

	if err != nil {
		return &pb.ChangeGroupNameResponse{
			Result: &pb.Result{Code: errorcode.GroupUserDatabaseError, Msg: fmt.Sprintf("Insert error: %v", err)}}, nil
	}

	if insertResp.Result.Code != errorcode.Success {
		return &pb.ChangeGroupNameResponse{
			Result: &pb.Result{Code: insertResp.Result.Code, Msg: insertResp.Result.Msg},
		}, nil
	}
	go gateway.ExecRedisDel(&pb_gtw.RedisDelRequest{DBID: 0, Key: "groupuser:public:" + fmt.Sprintf("%d", req.GroupId)})
	return &pb.ChangeGroupNameResponse{
		Result: &pb.Result{Code: errorcode.Success, Msg: ""},
	}, nil
}

// ChangeGroupPassword 设置群名密码
func (s *server) ChangeGroupPassword(ctx context.Context, req *pb.ChangeGroupPasswordRequest) (*pb.ChangeGroupPasswordResponse, error) {
	username, err := user.QueryUsernameByUID(ctx, req.Uid)
	if err != nil {
		return &pb.ChangeGroupPasswordResponse{
			Result: &pb.Result{Code: errorcode.GroupUserQueryError, Msg: fmt.Sprintf("User query error: %v", err)},
		}, nil
	}

	resp, err := gateway.ExecRedisBGet(&pb_gtw.RedisGetBytesRequest{DBID: 0, Key: "groupuser:info:" + fmt.Sprintf("%d", req.GroupId)})
	cacheObj := &pb.GetGroupInfoCache{}
	if err != nil || resp.Result.Code != errorcode.Success || len(resp.Value) == 0 || (proto.Unmarshal(resp.Value, cacheObj) != nil) {
		for { // 利用控制流跳出
			sqlReq := &pb_gtw.SqlRequest{
				Sql: "SELECT `username`, CAST(`type` AS CHAR) FROM `group_user_table` WHERE groupid = ?",
				Db:  pb_gtw.SqlDatabases_Groups,
				Params: []*pb_gtw.InterFaceType{
					{Response: &pb_gtw.InterFaceType_Int32{Int32: req.GroupId}},
				},
			}

			sqlResp, err := gateway.ExecSQL(sqlReq)
			if err != nil {
				return &pb.ChangeGroupPasswordResponse{
					Result: &pb.Result{Code: errorcode.GroupUserDatabaseError, Msg: fmt.Sprintf("Database error: %v", err)},
				}, nil
			}
			if sqlResp.Result.Code != errorcode.Success || len(sqlResp.Data) == 0 {
				break
			}

			for _, row := range sqlResp.Data {
				cacheObj.Members = append(cacheObj.Members, &pb.MemberObject{
					Name: row.Result[0].GetStr(),
					Type: convertSQLUserTypeToProto(row.Result[1].GetStr()),
				})
			}
			if true { // 保证始终跳出
				break
			}
		}
		cacheBytes, err := proto.Marshal(cacheObj)
		if err == nil {
			go gateway.ExecRedisBSet(&pb_gtw.RedisSetBytesRequest{DBID: 0, Key: "groupuser:info:" + fmt.Sprintf("%d", req.GroupId), Value: cacheBytes})
		}
	}
	if len(cacheObj.Members) == 0 {
		return &pb.ChangeGroupPasswordResponse{
			Result: &pb.Result{Code: errorcode.GroupUserNotFound, Msg: "Group not found"},
		}, nil
	}
	found := false
	for _, element := range cacheObj.Members {
		if element.Name == username {
			found = true
			if element.Type != pb.MemberType_owner && element.Type != pb.MemberType_manager {
				return &pb.ChangeGroupPasswordResponse{
					Result: &pb.Result{Code: errorcode.GroupUserPermissionDenied, Msg: "Permission denied"},
				}, nil
			}
		}
	}
	if !found {
		return &pb.ChangeGroupPasswordResponse{
			Result: &pb.Result{Code: errorcode.GroupUserPermissionDenied, Msg: "Permission denied"},
		}, nil
	}

	hashedPasswordRequest := sha256.Sum256([]byte(req.Password + config.LatestConfig.Security.PasswordSalt))
	insertReq := &pb_gtw.SqlRequest{
		Sql:    "UPDATE `groups` SET `password` = ? WHERE `groupid` = ?",
		Db:     pb_gtw.SqlDatabases_Groups,
		Commit: true,
		Params: []*pb_gtw.InterFaceType{
			{Response: &pb_gtw.InterFaceType_Str{Str: hex.EncodeToString(hashedPasswordRequest[:])}},
			{Response: &pb_gtw.InterFaceType_Int32{Int32: req.GroupId}},
		},
	}

	insertResp, err := gateway.ExecSQL(insertReq)

	if err != nil {
		return &pb.ChangeGroupPasswordResponse{
			Result: &pb.Result{Code: errorcode.GroupUserDatabaseError, Msg: fmt.Sprintf("Insert error: %v", err)}}, nil
	}

	if insertResp.Result.Code != errorcode.Success {
		return &pb.ChangeGroupPasswordResponse{
			Result: &pb.Result{Code: insertResp.Result.Code, Msg: insertResp.Result.Msg},
		}, nil
	}
	go gateway.ExecRedisDel(&pb_gtw.RedisDelRequest{DBID: 0, Key: "groupuser:public:" + fmt.Sprintf("%d", req.GroupId)})
	return &pb.ChangeGroupPasswordResponse{
		Result: &pb.Result{Code: errorcode.Success, Msg: ""},
	}, nil
}

// KickUser 踢出群成员
func (s *server) KickUser(ctx context.Context, req *pb.KickUserRequest) (*pb.KickUserResponse, error) {
	username, err := user.QueryUsernameByUID(ctx, req.Uid)
	if err != nil {
		return &pb.KickUserResponse{
			Result: &pb.Result{Code: errorcode.GroupUserQueryError, Msg: fmt.Sprintf("User query error: %v", err)},
		}, nil
	}

	resp, err := gateway.ExecRedisBGet(&pb_gtw.RedisGetBytesRequest{DBID: 0, Key: "groupuser:info:" + fmt.Sprintf("%d", req.GroupId)})
	cacheObj := &pb.GetGroupInfoCache{}
	if err != nil || resp.Result.Code != errorcode.Success || len(resp.Value) == 0 || (proto.Unmarshal(resp.Value, cacheObj) != nil) {
		for { // 利用控制流跳出
			sqlReq := &pb_gtw.SqlRequest{
				Sql: "SELECT `username`, CAST(`type` AS CHAR) FROM `group_user_table` WHERE groupid = ?",
				Db:  pb_gtw.SqlDatabases_Groups,
				Params: []*pb_gtw.InterFaceType{
					{Response: &pb_gtw.InterFaceType_Int32{Int32: req.GroupId}},
				},
			}

			sqlResp, err := gateway.ExecSQL(sqlReq)
			if err != nil {
				return &pb.KickUserResponse{
					Result: &pb.Result{Code: errorcode.GroupUserDatabaseError, Msg: fmt.Sprintf("Database error: %v", err)},
				}, nil
			}
			if sqlResp.Result.Code != errorcode.Success || len(sqlResp.Data) == 0 {
				break
			}

			for _, row := range sqlResp.Data {
				cacheObj.Members = append(cacheObj.Members, &pb.MemberObject{
					Name: row.Result[0].GetStr(),
					Type: convertSQLUserTypeToProto(row.Result[1].GetStr()),
				})
			}
			if true { // 保证始终跳出
				break
			}
		}
		cacheBytes, err := proto.Marshal(cacheObj)
		if err == nil {
			go gateway.ExecRedisBSet(&pb_gtw.RedisSetBytesRequest{DBID: 0, Key: "groupuser:info:" + fmt.Sprintf("%d", req.GroupId), Value: cacheBytes})
		}
	}
	if len(cacheObj.Members) == 0 {
		return &pb.KickUserResponse{
			Result: &pb.Result{Code: errorcode.GroupUserNotFound, Msg: "Group not found"},
		}, nil
	}
	found := false
	foundDist := false
	for _, element := range cacheObj.Members {
		switch element.Name {
		case username:
			found = true
			if element.Type != pb.MemberType_owner && element.Type != pb.MemberType_manager && req.Username != username {
				return &pb.KickUserResponse{
					Result: &pb.Result{Code: errorcode.GroupUserPermissionDenied, Msg: "Permission denied"},
				}, nil
			}
		case req.Username:
			foundDist = true
		}
	}
	if !found {
		return &pb.KickUserResponse{
			Result: &pb.Result{Code: errorcode.GroupUserPermissionDenied, Msg: "Permission denied"},
		}, nil
	}
	if !foundDist {
		return &pb.KickUserResponse{
			Result: &pb.Result{Code: errorcode.GroupUserNotFound, Msg: "User not found"},
		}, nil
	}

	insertReq := &pb_gtw.SqlRequest{
		Sql:    "DELETE FROM `group_user_table` WHERE `groupid` = ? AND `username` = ?",
		Db:     pb_gtw.SqlDatabases_Groups,
		Commit: true,
		Params: []*pb_gtw.InterFaceType{
			{Response: &pb_gtw.InterFaceType_Int32{Int32: req.GroupId}},
			{Response: &pb_gtw.InterFaceType_Str{Str: req.Username}},
		},
	}

	insertResp, err := gateway.ExecSQL(insertReq)

	if err != nil {
		return &pb.KickUserResponse{
			Result: &pb.Result{Code: errorcode.GroupUserDatabaseError, Msg: fmt.Sprintf("Insert error: %v", err)}}, nil
	}

	if insertResp.Result.Code != errorcode.Success {
		return &pb.KickUserResponse{
			Result: &pb.Result{Code: insertResp.Result.Code, Msg: insertResp.Result.Msg},
		}, nil
	}
	go gateway.ExecRedisDel(&pb_gtw.RedisDelRequest{DBID: 0, Key: "groupuser:info:" + fmt.Sprintf("%d", req.GroupId)})
	go func() {
		userID, err := user.QueryUIDByUsername(ctx, req.Username)
		if err != nil {
			return
		}
		gateway.ExecRedisDel(&pb_gtw.RedisDelRequest{DBID: 0, Key: "groupuser:groups:" + fmt.Sprintf("%d", userID)})
	}()
	return &pb.KickUserResponse{
		Result: &pb.Result{Code: errorcode.Success, Msg: ""},
	}, nil
}
