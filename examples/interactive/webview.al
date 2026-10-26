// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

// Desktop WebView Application
// Interactive example: opens a WebView window and blocks until it is closed.
use std.gui;

html := """
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: linear-gradient(135deg, #1e1e2e, #11111b);
            color: #cdd6f4;
            display: flex;
            flex-direction: column;
            align-items: center;
            justify-content: center;
            height: 100vh;
            margin: 0;
        }
        .card {
            background: rgba(255, 255, 255, 0.05);
            padding: 40px;
            border-radius: 16px;
            box-shadow: 0 8px 32px rgba(0,0,0,0.4);
            border: 1px solid rgba(255,255,255,0.1);
            text-align: center;
            max-width: 500px;
        }
        h1 { color: #89b4fa; margin-bottom: 8px; }
        p { color: #a6adc8; line-height: 1.6; }
        .btn {
            background: #89b4fa;
            color: #11111b;
            border: none;
            padding: 12px 28px;
            border-radius: 8px;
            font-weight: bold;
            font-size: 15px;
            cursor: pointer;
            transition: 0.2s;
            margin-top: 15px;
        }
        .btn:hover { background: #b4befe; transform: translateY(-2px); }
    </style>
</head>
<body>
    <div class="card">
        <h1>🚀 Alca Desktop WebView</h1>
        <p>Современное десктопное приложение на HTML/CSS с нулевыми зависимостями через встроенный движок Alca.</p>
        <button class="btn" onclick="alert('Привет из JavaScript внутри Alca!')">Нажми меня</button>
    </div>
</body>
</html>
""";

println("Открываю десктопное окно WebView...");
app := gui.webview("Alca Desktop App", html, 800, 550);
println("WebView окно открыто! Сервер запущен на: " + string(app["url"]));
println("Закройте окно приложения для завершения работы...");
app.wait();
println("WebView окно закрыто.");
