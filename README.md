# WebTerminal
A web-based terminal to interact with server shell through the browser, featuring real-time WebSocket communication and xterm.js terminal emulation.

# Demo
Visit http://localhost:8089 in your browser to see the web terminal connected to the server shell.

# Installation and Running
1. Clone the repository:
```shell
git clone https://github.com/garfcat/webterminal.git
cd webterminal
```
2. Install dependencies:
```shell
go mod tidy
```
3. Run the Go server:
```shell
go run main.go
```

4. Visit http://localhost:8089 in your browser.


# Security Notice
Since this project provides direct access to the server shell, it is essential to ensure proper authentication and authorization when used in a production environment.
