module github.com/nerdynz/flow

go 1.14

replace github.com/nerdynz/security => ../security

require (
	github.com/cznic/ql v1.2.0 // indirect
	github.com/go-zoo/bone v1.3.0
	github.com/google/martian v2.1.0+incompatible
	github.com/nerdynz/dat v1.3.0 // indirect
	github.com/nerdynz/datastore v0.0.0-20200402045006-0f63cc077d94
	github.com/nerdynz/security v0.0.0-20200722093232-8a8c4c983b6c
	github.com/nerdynz/view v0.0.0-20170422022719-673f2075b045
	github.com/oklog/ulid v1.3.1
	github.com/sirupsen/logrus v1.6.0
	github.com/unrolled/render v1.0.3
	google.golang.org/appengine v1.6.6 // indirect
	gopkg.in/mattes/migrate.v1 v1.3.2 // indirect
	gopkg.in/stretchr/testify.v1 v1.2.2 // indirect
)
