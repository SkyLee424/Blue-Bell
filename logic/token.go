package logic

import (
	"bluebell/dao/redis"
	bluebell "bluebell/errors"
	"bluebell/internal/utils"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pkg/errors"
)

// 使用 refreshToken 刷新（获取）accessToken
func RefreshToken(refresh_token, access_token string) (string, error) {
	jwtKey := utils.GetJwtKey()

	// 校验 refresh_token 是否有效（包括是否过期）
	_, err := jwt.ParseWithClaims(refresh_token, &jwt.RegisteredClaims{}, func(t *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return "", bluebell.ErrExpiredToken
		}
		return "", bluebell.ErrInvalidToken
	}

	usrClaims := new(utils.UserClaims)
	_, err = jwt.ParseWithClaims(access_token, usrClaims, func(t *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	})

	// 检验 access token 是否过期
	if err != nil && errors.Is(err, jwt.ErrTokenExpired) {
		// 过期，生成新的 access token
		// 为了判断登录状态是否过期，检验 refresh token 是否与 redis 中的一致
		rdb_refresh_token, err := redis.GetUserRefreshToken(usrClaims.UserID)
		if err != nil || rdb_refresh_token != refresh_token {
			return "", bluebell.ErrExpiredToken // refresh_token 不存在或者过期
		}

		return utils.GenToken(usrClaims.UserID, utils.AccessType)
	}

	return "", nil // 不需要更新
}

func SetUserAccessToken(userID int64, accessTokenStr string, expireDuration time.Duration) error {
	return redis.SetUserAccessToken(userID, accessTokenStr, expireDuration)
}

func GetUserAccessToken(userID int64) (string, error) {
	access_token, err := redis.GetUserAccessToken(userID)
	if err != nil {
		if err != redis.Nil {
			return "", errors.Wrap(err, "get user access_token")
		}
	}
	return access_token, nil
}
