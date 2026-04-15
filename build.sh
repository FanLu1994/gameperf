#!/bin/bash
# 一键构建脚本

set -e

echo "🔧 构建前端..."
cd web
npm install
npm run build
cd ..

echo "🏗️  构建后端..."
go mod tidy
go build -o gameperf .

echo "✅ 构建完成！"
echo "运行: ./gameperf"
echo "访问: http://localhost:9090"
