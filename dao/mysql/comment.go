package mysql

import (
	"bluebell/models"
	"fmt"

	"github.com/go-sql-driver/mysql"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

func CreateCommentContent(content *models.CommentContent) error {
	res := db.Create(&content)
	return errors.Wrap(res.Error, "mysql: CreateCommentContent failed")
}

func CreateCommentSubject(tx *gorm.DB, ID, objID int64, objType int8) error {
	useDB := getUseDB(tx)

	commentSubject := &models.CommentSubject{
		ID:        ID,
		ObjID:     objID,
		ObjType:   objType,
		Count:     0,
		RootCount: 0,
	}

	res := useDB.Create(&commentSubject)
	return errors.Wrap(res.Error, "mysql: CreateCommentSubject failed")
}

func CreateCommentLikeOrHateUser(tx *gorm.DB, CidUid string, like bool) error {
	useDB := getUseDB(tx)
	var res *gorm.DB

	if like {
		res = useDB.Create(&models.CommentLikeUser{
			CidUid: CidUid,
		})
	} else {
		res = useDB.Create(&models.CommentHateUser{
			CidUid: CidUid,
		})
	}

	// 忽略重复 key 的错误
	if res.Error != nil {
		rawErr, _ := res.Error.(*mysql.MySQLError)
		if rawErr.Number != 1062 {
			return errors.Wrap(res.Error, "mysql:CreateCommentLikeOrHateUser: Create")
		}
	}

	return nil
}

func CreateCommentIndex(tx *gorm.DB, index *models.CommentIndex) error {
	useDB := getUseDB(tx)

	res := useDB.Create(&index)
	return errors.Wrap(res.Error, "mysql: CreateCommentIndex failed")
}

// 递增 subject 的 count
func IncrCommentSubjectCountField(tx *gorm.DB, field string, objID int64, objType int8, offset int) error {
	if offset == 0 {
		return nil
	}
	useDB := getUseDB(tx)
	expr := fmt.Sprintf("%s + %d", field, offset)
	res := useDB.Model(&models.CommentSubject{}).Where("obj_id = ? AND obj_type = ?", objID, objType).Update(field, gorm.Expr(expr))
	return errors.Wrap(res.Error, "mysql: IncrCommentSubjectField failed")
}

// 递增根评论的 count
func IncrCommentIndexCountField(tx *gorm.DB, field string, id int64, offset int) error {
	if offset == 0 {
		return nil
	}
	useDB := getUseDB(tx)
	expr := fmt.Sprintf("%s + %d", field, offset)
	res := useDB.Model(&models.CommentIndex{}).Where("id = ?", id).Update(field, gorm.Expr(expr))
	return errors.Wrap(res.Error, "mysql: IncrCommentIndexCount failed")
}

func SelectCommentSubjectCountField(tx *gorm.DB, field string, objID int64, objType int8) (int, error) {
	useDB := getUseDB(tx)
	var count int
	res := useDB.Model(&models.CommentSubject{}).Select(field).Where("obj_id = ? AND obj_type = ?", objID, objType).Scan(&count)
	return count, errors.Wrap(res.Error, "mysql: SelectCommentSubjectCountField failed")
}

func SelectCommentIndexCountField(tx *gorm.DB, field string, root int64) (int, error) {
	useDB := getUseDB(tx)
	var count int
	res := useDB.Model(&models.CommentIndex{}).Select(field).Where("id = ?", root).Scan(&count)
	return count, errors.Wrap(res.Error, "mysql: SelectCommentIndexCountField")
}

// 根据 obj_id、obj_type 检查某个 subject 是否存在
func SelectCommentSubjectCount(tx *gorm.DB, objID int64, objType int8) (int, error) {
	useDB := getUseDB(tx)
	var count int
	res := useDB.Model(&models.CommentIndex{}).Select("count(*)").Where("obj_id = ? AND obj_type = ?", objID, objType).Scan(&count)
	return count, errors.Wrap(res.Error, "mysql: SelectCommentSubjectCount")
}

// 获取根评论 ID，不用排序也可以保证按楼层升序排列
func SelectRootCommentIDs(tx *gorm.DB, objType int8, objID int64) ([]int64, error) {
	useDB := getUseDB(tx)
	rootCommentIDs := make([]int64, 0)
	res := useDB.Model(&models.CommentIndex{}).Select("id").Where("root = 0 and obj_type = ? and obj_id = ?", objType, objID).
		Scan(&rootCommentIDs)
	return rootCommentIDs, errors.Wrap(res.Error, "mysql: SelectCommentIDs")
}

func SelectSubCommentIDs(tx *gorm.DB, root int64) ([]int64, error) {
	useDB := getUseDB(tx)
	subCommentIDs := make([]int64, 0)
	res := useDB.Model(&models.CommentIndex{}).Select("id").Where("root = ?", root).
		Scan(&subCommentIDs)
	return subCommentIDs, errors.Wrap(res.Error, "mysql: SelectSubCommentIDs")
}

func SelectFloorsByCommentIDs(tx *gorm.DB, commentIDs []int64) ([]int, error) {
	useDB := getUseDB(tx)
	floors := make([]int, 0)
	res := useDB.Model(&models.CommentIndex{}).Select("floor").Where("id in ?", commentIDs).
		Scan(&floors)
	return floors, errors.Wrap(res.Error, "mysql: SelectFloorsByCommentIDs")
}

func SelectCommentMetaDataByCommentIDs(tx *gorm.DB, commentIDs []int64) ([]models.CommentDTO, error) {
	useDB := getUseDB(tx)

	tmp := make([]models.CommentIndex, 0)

	res := useDB.Model(&models.CommentIndex{}).Where("id in ?", commentIDs).Scan(&tmp)

	if res.Error != nil {
		return nil, errors.Wrap(res.Error, "mysql: SelectCommentMetaDataByCommentIDs")
	}

	return selectCommentContentHelper(tmp), nil
}

func SelectCommentContentByCommentIDs(tx *gorm.DB, commentIDs []int64) ([]models.CommentContentDTO, error) {
	useDB := getUseDB(tx)

	contents := make([]models.CommentContentDTO, 0)

	res := useDB.Model(&models.CommentContent{}).Where("comment_id in ?", commentIDs).Scan(&contents)

	return contents, errors.Wrap(res.Error, "mysql: SelectCommentContentByCommentIDs")
}

// 根据传入的根评论 ID，查所有 reply 的元数据（不包括 content）
func SelectCommentReplyMetaDataByCommentIDs(tx *gorm.DB, commentIDs []int64) ([]models.CommentDTO, error) {
	useDB := getUseDB(tx)
	tmp := make([]models.CommentIndex, 0)

	res := useDB.Model(&models.CommentIndex{}).Where("root in ?", commentIDs).Scan(&tmp)

	if res.Error != nil {
		return nil, errors.Wrap(res.Error, "mysql: SelectCommentMetaDataByCommentIDs")
	}

	return selectCommentContentHelper(tmp), nil
}

func selectCommentContentHelper(tmp []models.CommentIndex) []models.CommentDTO {
	comments := make([]models.CommentDTO, 0, len(tmp))
	for i := 0; i < len(tmp); i++ {
		comments = append(comments, models.CommentDTO{
			CommentID: tmp[i].ID,
			ObjID:     tmp[i].ObjID,
			Type:      tmp[i].ObjType,
			Root:      tmp[i].Root,
			Parent:    tmp[i].Parent,
			UserID:    tmp[i].UserID,
			Floor:     tmp[i].Floor,
			Like:      tmp[i].Like,
			CreatedAt: tmp[i].CreatedAt,
			UpdatedAt: tmp[i].UpdatedAt,
		})
	}
	return comments
}

func SelectUserIDByCommentID(tx *gorm.DB, commentID int64) (int64, error) {
	useDB := getUseDB(tx)

	var userID int64
	res := useDB.Model(&models.CommentIndex{}).Select("user_id").Where("id = ?", commentID).Scan(&userID)

	return userID, errors.Wrap(res.Error, "mysql: SelectUserIDByCommentID")
}

func CheckIsRootComment(tx *gorm.DB, commentID int64) (bool, error) {
	useDB := getUseDB(tx)

	var root int64
	res := useDB.Model(&models.CommentIndex{}).Select("root").Where("id = ?", commentID).Scan(&root)

	return root == 0, errors.Wrap(res.Error, "mysql: CheckIsRootComment")
}

func CheckCidUidIfExist(tx *gorm.DB, CidUid string, like bool) (bool, error) {
	useDB := getUseDB(tx)
	var res *gorm.DB
	var count int
	if like {
		res = useDB.Model(&models.CommentLikeUser{}).Select("count(*)").Where("cid_uid = ?", CidUid).Scan(&count)
	} else {
		res = useDB.Model(&models.CommentHateUser{}).Select("count(*)").Where("cid_uid = ?", CidUid).Scan(&count)
	}
	return count == 1, errors.Wrap(res.Error, "mysql: CheckCidUidIfExist")
}

func SelectSubCommentIDsByField(tx *gorm.DB, commentID int64, field string) ([]int64, error) {
	useDB := getUseDB(tx)
	subCommentIDs := make([]int64, 0)

	res := useDB.Model(&models.CommentIndex{}).Select("id").Where(gorm.Expr(field+" = ?", commentID)).Scan(&subCommentIDs)

	return subCommentIDs, errors.Wrap(res.Error, "mysql: SelectSubCommentIDsByField")
}

func SelectCommentRootIDByCommentID(tx *gorm.DB, commentID int64) (int64, error) {
	useDB := getUseDB(tx)
	var root int64

	res := useDB.Model(&models.CommentIndex{}).Select("root").Where("id = ?", commentID).Scan(&root)
	return root, errors.Wrap(res.Error, "mysql: SelectCommentRootIDByCommentID")
}

func DeleteCommentIndexByCommentIDs(tx *gorm.DB, commentIDs []int64) error {
	useDB := getUseDB(tx)
	res := useDB.Delete(&models.CommentIndex{}, commentIDs)
	return errors.Wrap(res.Error, "mysql: DeleteCommentIndexByCommentIDs")
}

func DeleteCommentContentByCommentIDs(tx *gorm.DB, commentIDs []int64) error {
	useDB := getUseDB(tx)
	res := useDB.Delete(&models.CommentContent{}, commentIDs)
	return errors.Wrap(res.Error, "mysql: DeleteCommentContentByCommentIDs")
}

func DeleteCommentLikeUserByCidUids(tx *gorm.DB, ciduids []string) error {
	useDB := getUseDB(tx)
	res := useDB.Delete(&models.CommentLikeUser{}, "cid_uid in ?", ciduids)
	return errors.Wrap(res.Error, "mysql: DeleteCommentLikeUserByCidUids")
}

func DeleteCommentHateUserByCidUids(tx *gorm.DB, ciduids []string) error {
	useDB := getUseDB(tx)
	res := useDB.Delete(&models.CommentHateUser{}, "cid_uid in ?", ciduids)
	return errors.Wrap(res.Error, "mysql: DeleteCommentHateUserByCidUids")
}