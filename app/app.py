#!/usr/bin/env python3
"""示例 Python 应用 - 演示 ENV 变量在应用中的可用性"""
import os
import json
from http.server import HTTPServer, BaseHTTPRequestHandler

APP_NAME = os.environ.get("APP_NAME", "unknown")
APP_VERSION = os.environ.get("APP_VERSION", "0.0.0")
WEB_PORT = int(os.environ.get("WEB_PORT", "8080"))


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        response = {
            "app": APP_NAME,
            "version": APP_VERSION,
            "port": WEB_PORT,
            "path": self.path,
        }
        self.wfile.write(json.dumps(response, ensure_ascii=False).encode())

    def log_message(self, fmt, *args):
        print(f"[{APP_NAME}] {args[0]}")


if __name__ == "__main__":
    print(f"Starting {APP_NAME} v{APP_VERSION} on :{WEB_PORT}")
    HTTPServer(("0.0.0.0", WEB_PORT), Handler).serve_forever()
