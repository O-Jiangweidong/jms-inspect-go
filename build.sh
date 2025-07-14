#!/bin/bash


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
    CGO_ENABLED=0 GOOS=${goos} GOARCH=${arch} go build -o ${output_binary} ${base_dir}/pkg/cmd/inspect.go

    if [ $? -ne 0 ]; then
        echo "编译失败，请检查错误信息。"
        exit 1
    fi

    local temp_dir=$(mktemp -d)
    mkdir -p "${temp_dir}/config"
    cp "${output_binary}" "${temp_dir}/"
    cp "${base_dir}/config/machine-demo.csv" "${temp_dir}/config/"
    cp "${base_dir}/config/machine-demo.yml" "${temp_dir}/config/"

    (cd "${temp_dir}" && zip -r "${base_dir}/${zip_file}" .)

    if [ $? -eq 0 ]; then
        mv "${base_dir}/${zip_file}" "${release_dir}/"
        rm -rf "${temp_dir}" "${output_binary}"
        echo "${goos}-${arch} 版本脚本编译完成，生成的 ZIP 文件为 ${release_dir}/${zip_file}"
    else
        echo "压缩失败，请检查错误信息。"
        rm -rf "${temp_dir}"
        exit 1
    fi
}

change_version() {
    local version=$1
    if [ -n "$version" ]; then
        local sed_file="${base_dir}/pkg/cmd/inspect.go"
        if [[ $(uname) == "Darwin" ]]; then
            sed -i '' "s/const version = \"dev\"/const version = \"$version\"/" "$sed_file"
        else
            sed -i "s/const version = \"dev\"/const version = \"$version\"/" "$sed_file"
        fi
    fi
}

create_dirs() {
    release_dir="${base_dir}/release"
    mkdir -p "$release_dir"
}

compile() {
    local version=$1
    local os_arch_combinations=(
        "darwin amd64"
        "darwin arm64"
        "linux amd64"
        "linux arm64"
        "windows amd64"
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
    mkdir -p "$target_dir"
    if ! ls "$target_dir" | grep -qi "echarts"; then
        echo "未找到echarts文件，开始下载..."
        wget -P "$target_dir" "$echarts_url"
        if [ $? -eq 0 ]; then
            echo "下载成功，文件已保存到 $target_dir"
        else
            echo "下载失败，请检查网络或URL"
        fi
    else
        echo "echarts文件已存在，跳过下载"
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