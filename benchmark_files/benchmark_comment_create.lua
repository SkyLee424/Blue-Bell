wrk.method = "POST"
wrk.path = "/api/v1/comment/create"
wrk.body   = '{"message": "[吃瓜]1", "obj_id": "33120778639642624", "obj_type": 1, "parent": "0", "root": "0"}'
wrk.headers["Content-Type"] = "application/json"
wrk.headers["Accept"] = "application/json"
wrk.headers["Authorization"] = "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjo5NTQxNTExNTc4MzkwNTI4LCJpc3MiOiJTa3lfTGVlIiwiZXhwIjoxNzA1OTk1MDE1fQ.dgtp0lCEbG_0MUdI7gWq6kpcH7oJiD2XvDo0oZ5PTVQ"