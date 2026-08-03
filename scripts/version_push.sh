VERSION=v0.0.3-alpha

sed -i '' "s/\"productVersion\": \".*\"/\"productVersion\": \"$VERSION\"/" wails.json && \
sed -i '' "s/const appVersion = \".*\"/const appVersion = \"$VERSION\"/" app.go && \
git add wails.json app.go .github/workflows/release.yml && \
git commit -m "Bump version to $VERSION" && \
git tag $VERSION && \
git push origin main && \
git push origin $VERSION
