#!bash

package=${1:-ftpxmltopdf}
platforms=("windows/amd64" "darwin/amd64" "linux/amd64" "darwin/arm64")
output_dir="dist"
rm -rf $output_dir
mkdir -p "$output_dir"
export GIT_COMMIT=$(git rev-list -1 HEAD)

for platform in "${platforms[@]}"
do
	platform_split=(${platform//\// })
	GOOS=${platform_split[0]}
	GOARCH=${platform_split[1]}
	output_name=$package
	if [ $GOOS = "windows" ]; then
		output_name+='.exe'
	fi	

	env GOOS=$GOOS GOARCH=$GOARCH go build -ldflags "-X main.SecretKey=$GIT_COMMIT"  -o $output_name $package
	if [ $? -ne 0 ]; then
   		echo 'An error has occurred! Aborting the script execution...'
		exit 1
	fi
    chmod +x $output_name
	tar -zcf "$output_dir/"$output_name'-'$GOOS'-'$GOARCH'.gz' --exclude=".DS_Store" $output_name README.md templates
	rm $output_name
done