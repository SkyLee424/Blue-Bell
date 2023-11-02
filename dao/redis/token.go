package redis

import (
	"strconv"
	"time"

	"github.com/pkg/errors"
)

func SetUserAccessToken(userID int64, accessTokenStr string, expireDuration time.Duration) error {
	err := set(KeyAccessTokenStringPF+strconv.FormatInt(userID, 10), accessTokenStr, expireDuration)
	return errors.Wrap(err, "set access_token")
}

func GetUserAccessToken(userID int64) (string, error) {
	cmd := get(KeyAccessTokenStringPF + strconv.FormatInt(userID, 10))
	return cmd.Val(), errors.Wrap(cmd.Err(), "get access_token")
}

func SetUserRefreshToken(userID int64, refreshTokenStr string, expireDuration time.Duration) error {
	err := set(KeyRefreshTokenStringPF+strconv.FormatInt(userID, 10), refreshTokenStr, expireDuration)
	return errors.Wrap(err, "set refresh_token")
}

func GetUserRefreshToken(userID int64) (string, error) {
	cmd := get(KeyRefreshTokenStringPF + strconv.FormatInt(userID, 10))
	return cmd.Val(), errors.Wrap(cmd.Err(), "get refresh_token")
}
