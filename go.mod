module github.com/nerdynz/flow

go 1.18

replace github.com/nerdynz/security => ../security

replace github.com/nerdynz/datastore => ../datastore

require (
	github.com/go-zoo/bone v1.3.0
	github.com/nerdynz/datastore v0.0.0-20210404043820-fca6c2b865be
	github.com/nerdynz/security v0.0.0-20200722094918-c9da0af68175
	github.com/oklog/ulid v1.3.1
	github.com/unrolled/render v1.4.1
)

require (
	github.com/cenkalti/backoff v2.2.1+incompatible // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/fsnotify/fsnotify v1.5.4 // indirect
	github.com/garyburd/redigo v1.6.3 // indirect
	github.com/jmoiron/sqlx v1.3.5 // indirect
	github.com/lib/pq v1.10.6 // indirect
	github.com/mgutz/str v1.2.0 // indirect
	github.com/nerdynz/dat v1.3.0 // indirect
	github.com/pmylund/go-cache v2.1.0+incompatible // indirect
	github.com/satori/go.uuid v1.2.0 // indirect
	golang.org/x/sys v0.0.0-20220520151302-bc2c85ada10a // indirect
)
