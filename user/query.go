package user

import (
	pb "StealthIMGroupUser/StealthIM.User"
	"StealthIMGroupUser/config"
	"context"
	"fmt"
	"time"
)

// QueryUsernameByUID 通过 uid 查询用户名
func QueryUsernameByUID(ctx context.Context, uid int32) (string, error) {
	mainlock.RLock()
	defer mainlock.RUnlock()
	conn, err := chooseConn()
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(config.LatestConfig.DBGateway.Timeout)*time.Millisecond)
	defer cancel()
	c := pb.NewStealthIMUserClient(conn)
	res, err2 := c.GetUsernameByUID(ctx, &pb.GetUsernameByUIDRequest{UserId: uid})
	if err2 != nil {
		return "", err2
	}
	if res.Result.Code != 0 {
		return "", fmt.Errorf("[%d]%s", res.Result.Code, res.Result.Msg)
	}
	return res.Username, err2
}

// QueryHasUsername 查询用户名是否存在
func QueryHasUsername(ctx context.Context, username string) bool {
	mainlock.RLock()
	defer mainlock.RUnlock()
	conn, err := chooseConn()
	if err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(config.LatestConfig.DBGateway.Timeout)*time.Millisecond)
	defer cancel()
	c := pb.NewStealthIMUserClient(conn)
	res, err2 := c.GetOtherUserInfo(ctx, &pb.GetOtherUserInfoRequest{Username: username})
	if err2 != nil {
		return false
	}
	if res.Result.Code != 0 {
		return false
	}
	return true
}

// QueryUIDByUsername 通过用户名查询 uid
func QueryUIDByUsername(ctx context.Context, username string) (int32, error) {
	mainlock.RLock()
	defer mainlock.RUnlock()
	conn, err := chooseConn()
	if err != nil {
		return 0, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(config.LatestConfig.DBGateway.Timeout)*time.Millisecond)
	defer cancel()
	c := pb.NewStealthIMUserClient(conn)
	res, err2 := c.GetUIDByUsername(ctx, &pb.GetUIDByUsernameRequest{Username: username})
	if err2 != nil {
		return 0, err2
	}
	if res.Result.Code != 0 {
		return 0, fmt.Errorf("[%d]%s", res.Result.Code, res.Result.Msg)
	}
	return res.UserId, err2
}
