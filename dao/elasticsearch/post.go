package elasticsearch

import (
	bluebell "bluebell/errors"
	"bluebell/logger"
	"bluebell/models"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/elastic/go-elasticsearch/v8/typedapi/core/search"
	"github.com/pkg/errors"
)

func CreatePost(doc *models.PostDoc) error {
	ctx := context.Background() // 后面根据需要修改
	_, err := clnt.Index("bluebell_post_index").
		Id(strconv.FormatInt(doc.PostID, 10)).
		Document(&doc).
		Do(ctx)

	if err != nil {
		return errors.Wrap(err, "elasticsearch: create post failed")
	}
	return nil
}

func GetPostDetail(PostID int64) (*models.PostDoc, error) {
	ctx := context.Background()
	resp, err := clnt.Get("bluebell_post_index", strconv.FormatInt(PostID, 10)).Do(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "elasticsearch: get post failed")
	}
	if !resp.Found {
		return nil, bluebell.ErrNoSuchPost
	}
	doc := new(models.PostDoc)
	if err := json.Unmarshal(resp.Source_, &doc); err != nil {
		return nil, errors.Wrap(err, "json: unmarshal post failed")
	}
	return doc, nil
}

func GetPostIDsByKeywordOrderByCorrelation(params *models.ParamPostListByKeyword) ([]string, int, error) {
	query := fmt.Sprintf(`{
		"query": {
		  "function_score": {
			"query": {
			  "bool": {
				"should": [
				  {
					"match": {
					  "title": "%s"
					}
				  },
				  {
					"match": {
					  "content": "%s"
					}
				  }
				]
			  }
			},
			"functions": [
			  {
				"filter": {
				  "range": {
					"created_time": {
					  "gte": "now-%dd/d",
					  "lte": "now/d"
					}
				  }
				},
				"weight": 3
			  }
			],
			"boost_mode": "multiply"
		  }
		},
		"size": 0,
		"aggs": {
		  "post_ids": {
			"terms": {
			  "field": "post_id",
			  "order": {
				"max_score": "desc"
			  }
			},
			"aggs": {
			  "max_score": {
				"max": {
				  "script": {
					"source": "_score"
				  }
				}
			  }
			}
		  }
		}
	  }`, params.Keyword, params.Keyword, postActiveDay)
	return getPostIDsHelper(query, params.PageNum, params.PageSize)
}

func GetPostIDsByKeywordOrderByTime(params *models.ParamPostListByKeyword) ([]string, int, error) {
	query := fmt.Sprintf(`{
		"query": {
		  "bool": {
			"should": [
			  {"match": {"title": "%s"}},
			  {"match": {"content": "%s"}}
			]
		  }
		},
		"size": 0,
		"aggs": {
		  "post_ids": {
			"terms": {
			  "field": "post_id",
			  "order": [
				{"_created_time": "desc"},
				{"max_score": "desc"}
			  ]
			},
			"aggs": {
			  "max_score": {
				"max": {
				  "script": {
					"source": "_score"
				  }
				}
			  },
			  "_created_time": {
				"max": {
				  "field": "created_time"
				}
			  }
			}
		  }
		}
	  }`, params.Keyword, params.Keyword)
	return getPostIDsHelper(query, params.PageNum, params.PageSize)
}

func getPostIDsHelper(query string, pageNum, pageSize int64) ([]string, int, error) {
	ctx := context.Background()

	res, err := lowlevelClnt.Search(
		lowlevelClnt.Search.WithIndex("bluebell_post_index"),
		// lowlevelClnt.Search.WithFrom((int(pageNum - 1) * int(pageSize))), 	// 使用的是聚合桶内的数据，对查询进行分页，没有意义（因为根本就不需要使用查询的数据）
		// lowlevelClnt.Search.WithSize(int(pageSize)),						    // 在查询体内（RAW Json），会限制 size 为 0，提高性能
		lowlevelClnt.Search.WithBody(strings.NewReader(query)),
		lowlevelClnt.Search.WithContext(ctx),
		// lowlevelClnt.Search.WithFilterPath("aggregations.post_ids.buckets"), // 而不是使用 filterPath
	)
	if err != nil {
		return nil, 0, errors.Wrap(err, "elasticsearch: search post failed")
	}

	// 转换为 *search.Response 类型
	resp := new(search.Response)
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return nil, 0, errors.Wrap(err, "json: decoding the response failed")
	}

	// 解析聚合结果
	aggregations, found := resp.Aggregations["post_ids"]
	if !found {
		return nil, 0, errors.Wrap(bluebell.ErrInternal, "elasticsearch: post_ids aggregation not found")
	}
	// 将聚合结果转换为 map
	postIDsAgg := aggregations.(map[string]any)
	// 获取桶（buckets）列表
	buckets := postIDsAgg["buckets"]

	// 解析桶数据
	postIDs := make([]string, 0)
	for _, bucket := range buckets.([]any) {
		bucketData := bucket.(map[string]any)
		key, found := bucketData["key"]
		if !found {
			return nil, 0, errors.Wrap(bluebell.ErrInternal, "elasticsearch: key not found in post_ids bucket")
		}
		postID := key.(float64)
		postIDs = append(postIDs, strconv.FormatInt(int64(postID), 10))
	}

	start := (pageNum - 1) * pageSize
	end := start + pageSize
	if start != 0 && int(start) >= len(postIDs) { // 错误的分页参数（注意排除搜索结果为空的情况）
		logger.Debugf("start:%v, end:%v", start, end)
		return nil, 0, errors.Wrap(bluebell.ErrInvalidParam, "elasticsearch: wrong paging parameters")
	}
	if int(end) > len(postIDs) {
		end = int64(len(postIDs))
	}
	return postIDs[start:end], len(postIDs),nil
}

func UpdatePost(doc *models.PostDoc) error {
	ctx := context.Background()
	_, err := clnt.Update("bluebell_post_index", strconv.FormatInt(doc.PostID, 10)).Doc(doc).Do(ctx)
	return errors.Wrap(err, "elasticsearch: update failed")
}

func DeletePost(postID int64) error {
	ctx := context.Background()
	resp, err := clnt.Delete("bluebell_post_index", strconv.FormatInt(postID, 10)).Do(ctx)
	if resp.Result.Name == "not_found" {
		return errors.Wrap(bluebell.ErrInternal, "elasticsearch: no such post")
	}
	return errors.Wrap(err, "elasticsearch: delete failed")
}
