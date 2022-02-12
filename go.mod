module brandonplank.org/checkout

go 1.17

require (
	github.com/gofiber/fiber/v2 v2.26.0
	github.com/gofiber/template v1.6.22
)

require (
	brandonplank.org/checkout/models v0.0.0-00010101000000-000000000000 // indirect
	github.com/gocarina/gocsv v0.0.0-20211203214250-4735fba0c1d9 // indirect
)

replace (
	//brandonplank.org/checkout/global => ./Global
	brandonplank.org/checkout/models => ./Models
	brandonplank.org/checkout/routes => ./Routes
)

require (
	brandonplank.org/checkout/routes v0.0.0
	github.com/andybalholm/brotli v1.0.4 // indirect
	github.com/klauspost/compress v1.14.2 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasthttp v1.33.0 // indirect
	github.com/valyala/tcplisten v1.0.0 // indirect
	golang.org/x/sys v0.0.0-20220209214540-3681064d5158 // indirect
)
