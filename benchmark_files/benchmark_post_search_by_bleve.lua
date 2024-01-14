wrk.method = "GET"
wrk.headers["Accept"] = "application/json"
wrk.path = "/api/v1/post/search?keyword=test&orderby=correlation&page=1&size=20"