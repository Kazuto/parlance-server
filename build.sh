#!/bin/bash
set -e

# Configuration
REGISTRY="${REGISTRY:-ghcr.io}"
NAMESPACE="${NAMESPACE:-$(git config user.name | tr '[:upper:]' '[:lower:]')}"
VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo 'latest')}"
IMAGE_NAME="${REGISTRY}/${NAMESPACE}/parlance-api"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

usage() {
    cat << EOF
Usage: $0 [OPTIONS]

Build and optionally publish Docker image for Parlance API.

OPTIONS:
    -b, --build         Build image only (default)
    -p, --push          Build and push image to registry
    -t, --tag TAG       Use specific tag (default: git describe)
    -r, --registry URL  Container registry URL (default: ghcr.io)
    -n, --namespace NS  Registry namespace (default: git username)
    -h, --help          Show this help message

EXAMPLES:
    # Build image locally
    ./build.sh --build

    # Build and push to GitHub Container Registry
    ./build.sh --push

    # Build and push with custom tag
    ./build.sh --push --tag v1.0.0

    # Build and push to Docker Hub
    ./build.sh --push --registry docker.io --namespace yourusername

EOF
}

build_image() {
    print_info "Building API image with tag: ${VERSION}"

    docker build -t "${IMAGE_NAME}:${VERSION}" \
        -t "${IMAGE_NAME}:latest" \
        --target runtime \
        -f docker/Dockerfile \
        .

    print_info "Build complete!"
    echo ""
    echo "Images built:"
    echo "  - ${IMAGE_NAME}:${VERSION}"
    echo "  - ${IMAGE_NAME}:latest"
}

push_image() {
    print_info "Pushing images to ${REGISTRY}..."

    docker push "${IMAGE_NAME}:${VERSION}"
    docker push "${IMAGE_NAME}:latest"

    print_info "Push complete!"
    echo ""
    echo "Images available at:"
    echo "  - ${IMAGE_NAME}:${VERSION}"
    echo "  - ${IMAGE_NAME}:latest"
}

# Parse arguments
PUSH=false
CUSTOM_TAG=""

while [[ $# -gt 0 ]]; do
    case $1 in
        -b|--build)
            PUSH=false
            shift
            ;;
        -p|--push)
            PUSH=true
            shift
            ;;
        -t|--tag)
            CUSTOM_TAG="$2"
            shift 2
            ;;
        -r|--registry)
            REGISTRY="$2"
            shift 2
            ;;
        -n|--namespace)
            NAMESPACE="$2"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            print_error "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
done

# Use custom tag if provided
if [ -n "$CUSTOM_TAG" ]; then
    VERSION="$CUSTOM_TAG"
fi

# Update image name with final values
IMAGE_NAME="${REGISTRY}/${NAMESPACE}/parlance-api"

# Main execution
print_info "Configuration:"
echo "  Registry:   ${REGISTRY}"
echo "  Namespace:  ${NAMESPACE}"
echo "  Version:    ${VERSION}"
echo "  Image:      ${IMAGE_NAME}"
echo ""

build_image

if [ "$PUSH" = true ]; then
    echo ""
    print_warn "You need to be logged in to the registry."
    print_info "Login with: docker login ${REGISTRY}"
    echo ""
    read -p "Push images to registry? (y/N) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        push_image
    else
        print_info "Skipping push."
    fi
fi
