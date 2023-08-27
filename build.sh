#!bash

package=${1:-ftpxmltopdf}
platforms=("windows/amd64" "darwin/amd64" "linux/amd64" "darwin/arm64")
# TODO: Check if there are open commits, check if version commits tag, commit changes, tag, update version and end commit

output_dir="dist"
rm -rf $output_dir
mkdir -p "$output_dir"
#Get the build version
if [ ! -f VERSION.txt ];then
  echo "0.0.1" > VERSION.txt
fi
export BUILD_VERSION=$(<VERSION.txt)

# Create build_key if it does not exits
if [ ! -f .build_key.txt ];then
  export BUILD_KEY=$(openssl rand -hex 16)
  echo -n "$BUILD_KEY" > .build_key.txt
fi
export BUILD_KEY=$(<.build_key.txt)

# Get the last git checking reversion
export GIT_BUILD=$(git rev-parse --short HEAD)

for platform in "${platforms[@]}"
do
	platform_split=(${platform//\// })
	GOOS=${platform_split[0]}
	GOARCH=${platform_split[1]}
	output_name=$package
	if [ $GOOS = "windows" ]; then
		output_name+='.exe'
	fi	

	env GOOS=$GOOS GOARCH=$GOARCH go build -ldflags="-X 'secure.SecretKey=$BUILD_KEY' -X 'main.BuildVersion=$GIT_BUILD' -X 'main.Version=$BUILD_VERSION' -X 'main.BaseName=$package'"  -o $output_name $package
	if [ $? -ne 0 ]; then
   		echo 'An error has occurred! Aborting the script execution...'
		exit 1
	fi
    chmod +x $output_name
	tar -zcf "$output_dir/"$output_name'-'$GOOS'-'$GOARCH'.gz' --exclude=".DS_Store" $output_name README.md templates
	rm $output_name
done