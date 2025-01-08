#!/bin/bash


# 定义函数
build_and_zip() {
    local goos=$1
    local output_binary=$2
    local zip_file="jms_inspect_${goos}.zip"

    echo "开始编译 ${goos} 版本脚本"
    bash -c "CGO_ENABLED=0 GOOS=${goos} GOARCH=amd64 go build -o ${output_binary} pkg/cmd/inspect.go"

    if [ $? -ne 0 ]; then
        echo "编译失败，请检查错误信息。"
        exit 1
    fi

    zip -r "${zip_file}" "${output_binary}" "${config_file}"

    if [ $? -eq 0 ]; then
        mv "${zip_file}" "$release_dir/"
        rm "${output_binary}"
        echo -e "${goos} 版本脚本编译完成，生成的 ZIP 文件为 ${zip_file}\n"
    else
        echo "压缩失败，请检查错误信息。"
        exit 1
    fi
}

build() {
  config_file="config/machine-demo.csv"
  release_dir="release"
  mkdir -p "$release_dir"
  # Version
  if [ -n "$VERSION" ]; then
    sed -i '' "s/const version = \"dev\"/const version = \"$VERSION\"/" "pkg/cmd/inspect.go"
  fi
  # Mac
  build_and_zip "darwin" "jms_inspect"
  # Linux
  build_and_zip "linux" "jms_inspect"
  # Windows
  build_and_zip "windows" "jms_inspect.exe"
}

build "$@"