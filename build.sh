#!/bin/bash


# 定义函数
build_and_zip() {
    local goos=$1
    local arch=$2
    local output_binary=$3
    local version=$4
    local zip_file="jms_inspect_${goos}_${arch}"

    if [ -n "$version" ]; then
      zip_file="${zip_file}_${version}"
    fi
    zip_file="${zip_file}.zip"

    echo "开始编译 ${goos}-${arch} 版本脚本"
    bash -c "CGO_ENABLED=0 GOOS=${goos} GOARCH=${arch} go build -o ${output_binary} pkg/cmd/inspect.go"

    if [ $? -ne 0 ]; then
        echo "编译失败，请检查错误信息。"
        exit 1
    fi

    zip -r "${zip_file}" "${output_binary}" "${config1_file}" "${config2_file}"

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
  config1_file="config/machine-demo.csv"
  config2_file="config/machine-demo.yml"
  release_dir="release"
  mkdir -p "$release_dir"
  # Version
  if [ -n "$VERSION" ]; then
    if [[ $(uname) == "Darwin" ]]; then
      sed -i '' "s/const version = \"dev\"/const version = \"$VERSION\"/" "pkg/cmd/inspect.go"
    else
      sed -i "s/const version = \"dev\"/const version = \"$VERSION\"/" "pkg/cmd/inspect.go"
    fi
  fi

  os_arch_combinations=(
      "darwin amd64"
      "darwin arm64"
      "linux amd64"
      "linux arm64"
      "windows amd64"
      "windows arm64"
  )
  for combination in "${os_arch_combinations[@]}"; do
      IFS=' ' read -r goos arch <<< "$combination"
      if [ "$goos" = "windows" ]; then
          output_binary="jms_inspect.exe"
      else
          output_binary="jms_inspect"
      fi
      build_and_zip "$goos" "$arch" "$output_binary" "$VERSION"
  done
}

build "$@"
