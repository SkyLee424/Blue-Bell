package redis

import (
	"context"

	"github.com/pkg/errors"
)

// 给评论点赞（踩）的 lua 脚本
const LuaCommentLikeOrHate = `
-- 参数定义
local keyCommentUserLikeOrHateIDs = KEYS[1] -- bluebell:comment:userlikeids:（记录某个主题下，一个用户给哪些评论点过赞）
local keyCommentRemCidSet = KEYS[2]         -- bluebell:comment:rem:cid（记录待删除的 cid_uid）
local keyCommentLikeOrHateCount = KEYS[3]   -- bluebell:comment:like:（记录点赞数）
local commentID = ARGV[1]


-- 检查并执行点赞或取消点赞
local liked = redis.call("SISMEMBER", keyCommentUserLikeOrHateIDs, commentID)

if liked == 1 then
    -- 如果用户点赞过，执行取消点赞逻辑
    redis.call("SADD", keyCommentRemCidSet, commentID)
    redis.call("SREM", keyCommentUserLikeOrHateIDs, commentID)
    redis.call("INCRBY", keyCommentLikeOrHateCount, -1)
else
    -- 用户未点赞，执行点赞逻辑
    redis.call("SREM", keyCommentRemCidSet, commentID)
    redis.call("SADD", keyCommentUserLikeOrHateIDs, commentID)
    redis.call("INCRBY", keyCommentLikeOrHateCount, 1)
end

return {}
`

var (
	shaCommentLikeOrHate string
)

func UploadLuaScript() error {
	cmd := rdb.ScriptLoad(context.TODO(), LuaCommentLikeOrHate)
	if cmd.Err() != nil {
		return errors.Wrap(cmd.Err(), "redis:UploadLuaScript: ScriptLoad")
	}
	shaCommentLikeOrHate = cmd.Val()

	return nil
}
