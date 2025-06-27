#!/bin/bash

# Quick Release Script - Simplified version for fast releases
# Usage: ./quick-release.sh [patch|minor|major]

set -e

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m'

print_info() { echo -e "${BLUE}ℹ️  $1${NC}"; }
print_success() { echo -e "${GREEN}✅ $1${NC}"; }
print_error() { echo -e "${RED}❌ $1${NC}"; }

# Get latest tag
get_latest_tag() {
    git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0"
}

# Parse and increment version
increment_version() {
    local current=$1
    local type=$2
    
    # Remove 'v' prefix
    current=${current#v}
    
    # Parse version
    if [[ $current =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
        local major=${BASH_REMATCH[1]}
        local minor=${BASH_REMATCH[2]}
        local patch=${BASH_REMATCH[3]}
        
        case $type in
            "patch") ((patch++)) ;;
            "minor") ((minor++)); patch=0 ;;
            "major") ((major++)); minor=0; patch=0 ;;
            *) print_error "Invalid type: $type"; exit 1 ;;
        esac
        
        echo "v${major}.${minor}.${patch}"
    else
        print_error "Invalid version format: $current"
        exit 1
    fi
}

# Main logic
main() {
    local type=${1:-}
    
    if [[ -z "$type" ]]; then
        echo "Usage: $0 [patch|minor|major]"
        echo
        echo "Current version: $(get_latest_tag)"
        exit 1
    fi
    
    if [[ ! "$type" =~ ^(patch|minor|major)$ ]]; then
        print_error "Invalid type: $type. Use patch, minor, or major"
        exit 1
    fi
    
    local current=$(get_latest_tag)
    local new_version=$(increment_version "$current" "$type")
    
    print_info "Current: $current"
    print_info "New: $new_version ($type)"
    
    # Create and push tag
    git tag -a "$new_version" -m "Release $new_version"
    git push origin "$new_version"
    
    print_success "Released $new_version!"
}

main "$@"