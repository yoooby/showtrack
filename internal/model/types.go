package model

// here's another stupid idea, so the primary id is a hash(SHOWNAME + SEAOSN + EPISODE) and struct also has tmdb_ID now when we use tmdb enabled we will get next  episode based on tmdb id

type Episode struct {
	Id string
	Title string
	Season int
	Episode int
	Path string
}
