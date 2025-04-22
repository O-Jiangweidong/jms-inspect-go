#!/bin/bash


# 定义函数
build_and_zip() {
    local config1_file="${base_dir}/config/machine-demo.csv"
    local config2_file="${base_dir}/config/machine-demo.yml"

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
    bash -c "CGO_ENABLED=0 GOOS=${goos} GOARCH=${arch} go build -o ${output_binary} ${base_dir}/pkg/cmd/inspect.go"

    # shellcheck disable=SC2181
    if [ $? -ne 0 ]; then
        echo "编译失败，请检查错误信息。"
        exit 1
    fi

    zip -r "${zip_file}" "${output_binary}" "${config1_file}" "${config2_file}"

    if [ $? -eq 0 ]; then
        mv "${zip_file}" "${base_dir}/release/"
        rm "${output_binary}"
        echo -e "${goos} 版本脚本编译完成，生成的 ZIP 文件为 ${zip_file}\n"
    else
        echo "压缩失败，请检查错误信息。"
        exit 1
    fi
}

change_version() {
  local version=$1
  if [ -n "$version" ]; then
    if [[ $(uname) == "Darwin" ]]; then
      sed -i '' "s/const version = \"dev\"/const version = \"$version\"/" "pkg/cmd/inspect.go"
    else
      sed -i "s/const version = \"dev\"/const version = \"$VERSION\"/" "pkg/cmd/inspect.go"
    fi
  fi
}

create_dirs() {
  release_dir="${base_dir}/release"
  mkdir -p "$release_dir"
}

compile() {
  local version=$1
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
      build_and_zip "$goos" "$arch" "$output_binary" "$version"
  done
}

download_dependencies() {
  local echarts_url="https://cdn.staticfile.net/echarts/5.4.1/echarts.min.js"
  local target_dir="${base_dir}/pkg/report/templates"
  if ! ls "$target_dir" | grep -qi "echarts"; then
      echo "未找到文件名包含 echarts 的文件，开始下载..."
      wget -P "$target_dir" "$echarts_url"
      if [ $? -eq 0 ]; then
          echo "下载成功，文件已保存到 $target_dir"
      else
          echo "下载失败，请检查网络或 URL 是否正确。"
      fi
  else
      echo "目录下已存在文件名包含 echarts 的文件，无需下载。"
  fi
}

build() {
  base_dir=$(dirname "$(realpath "$0")")
  create_dirs
  download_dependencies
  change_version "$VERSION"
  compile "$VERSION"
}

build "$@"
