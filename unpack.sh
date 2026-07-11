#!/bin/bash

set -e

# 检查是否提供了文件名参数
if [ -z "$1" ]; then
  echo "用法: $0 <文件名.tar.gz>"
  exit 1
fi

# 检查文件是否存在
if [ ! -f "$1" ]; then
  echo "错误: 文件 '$1' 未找到."
  exit 1
fi

echo "正在从 $1 解压文件并覆盖现有文件..."
# 解压文件，覆盖同名文件
tar -xzf "$1"
echo "解压完成。"

echo "正在删除源文件 $1..."
rm "$1"
echo "源文件删除完成。"