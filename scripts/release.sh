#!/bin/bash

# DVOM Release Management Script
# This script helps manage semantic versioning and creates new git tags

set -e  # Exit on any error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

# Function to check if we're in a git repository
check_git_repo() {
    if ! git rev-parse --git-dir > /dev/null 2>&1; then
        print_error "Not in a git repository!"
        exit 1
    fi
}

# Function to check if working directory is clean
check_clean_working_dir() {
    if [[ -n $(git status --porcelain) ]]; then
        print_warning "Working directory is not clean!"
        git status --short
        echo
        read -p "Continue anyway? (y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            print_info "Aborting release"
            exit 0
        fi
    fi
}

# Function to get the latest tag
get_latest_tag() {
    # Get the latest tag, handling case where no tags exist
    local latest_tag=$(git describe --tags --abbrev=0 2>/dev/null || echo "")
    
    if [[ -z "$latest_tag" ]]; then
        echo "v0.0.0"
    else
        echo "$latest_tag"
    fi
}

# Function to parse semantic version
parse_version() {
    local version=$1
    # Remove 'v' prefix if present
    version=${version#v}
    
    # Split version into components
    if [[ $version =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
        MAJOR=${BASH_REMATCH[1]}
        MINOR=${BASH_REMATCH[2]}
        PATCH=${BASH_REMATCH[3]}
        return 0
    else
        print_error "Invalid semantic version format: $version"
        print_info "Expected format: MAJOR.MINOR.PATCH (e.g., 1.2.3)"
        return 1
    fi
}

# Function to increment version
increment_version() {
    local type=$1
    
    case $type in
        "patch")
            ((PATCH++))
            ;;
        "minor")
            ((MINOR++))
            PATCH=0
            ;;
        "major")
            ((MAJOR++))
            MINOR=0
            PATCH=0
            ;;
        *)
            print_error "Invalid version type: $type"
            return 1
            ;;
    esac
}

# Function to create and push new tag
create_tag() {
    local new_version=$1
    local tag_message=$2
    
    print_info "Creating tag: $new_version"
    
    # Create annotated tag
    if git tag -a "$new_version" -m "$tag_message"; then
        print_success "Tag created successfully: $new_version"
        
        # Ask if user wants to push the tag
        echo
        read -p "Push tag to remote? (Y/n): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Nn]$ ]]; then
            print_warning "Tag created locally but not pushed to remote"
            print_info "To push later, run: git push origin $new_version"
        else
            print_info "Pushing tag to remote..."
            if git push origin "$new_version"; then
                print_success "Tag pushed to remote successfully"
            else
                print_error "Failed to push tag to remote"
                return 1
            fi
        fi
    else
        print_error "Failed to create tag"
        return 1
    fi
}

# Function to show version preview
show_version_preview() {
    local current=$1
    
    # Parse current version
    if ! parse_version "$current"; then
        return 1
    fi
    
    # Calculate incremented versions
    local orig_major=$MAJOR
    local orig_minor=$MINOR  
    local orig_patch=$PATCH
    
    # Patch version
    MAJOR=$orig_major
    MINOR=$orig_minor
    PATCH=$orig_patch
    increment_version "patch"
    local patch_version="v${MAJOR}.${MINOR}.${PATCH}"
    
    # Minor version
    MAJOR=$orig_major
    MINOR=$orig_minor
    PATCH=$orig_patch
    increment_version "minor"
    local minor_version="v${MAJOR}.${MINOR}.${PATCH}"
    
    # Major version
    MAJOR=$orig_major
    MINOR=$orig_minor
    PATCH=$orig_patch
    increment_version "major"
    local major_version="v${MAJOR}.${MINOR}.${PATCH}"
    
    # Reset to original values
    MAJOR=$orig_major
    MINOR=$orig_minor
    PATCH=$orig_patch
    
    echo
    echo "📋 Version Options:"
    echo "   Current: $current"
    echo "   Patch:   $patch_version (bug fixes)"
    echo "   Minor:   $minor_version (new features, backward compatible)"
    echo "   Major:   $major_version (breaking changes)"
    echo
}

# Function to get user input for version type
get_version_choice() {
    local current=$1
    
    # Parse current version to show exact increments
    if ! parse_version "$current"; then
        return 1
    fi
    
    # Calculate incremented versions for display
    local orig_major=$MAJOR
    local orig_minor=$MINOR  
    local orig_patch=$PATCH
    
    # Patch version
    MAJOR=$orig_major
    MINOR=$orig_minor
    PATCH=$orig_patch
    increment_version "patch"
    local patch_version="v${MAJOR}.${MINOR}.${PATCH}"
    
    # Minor version
    MAJOR=$orig_major
    MINOR=$orig_minor
    PATCH=$orig_patch
    increment_version "minor"
    local minor_version="v${MAJOR}.${MINOR}.${PATCH}"
    
    # Major version
    MAJOR=$orig_major
    MINOR=$orig_minor
    PATCH=$orig_patch
    increment_version "major"
    local major_version="v${MAJOR}.${MINOR}.${PATCH}"
    
    # Reset to original values
    MAJOR=$orig_major
    MINOR=$orig_minor
    PATCH=$orig_patch

    while true; do
        echo "🚀 Select the type of release:"
        echo
        echo "  1) 🐛 Patch Release → $patch_version"
        echo "     └─ Bug fixes, security patches, documentation updates"
        echo "     └─ No new features or breaking changes"
        echo
        echo "  2) ✨ Minor Release → $minor_version"  
        echo "     └─ New features, improvements, backward compatible changes"
        echo "     └─ What you want for: new encryption, progress bars, etc."
        echo
        echo "  3) 💥 Major Release → $major_version"
        echo "     └─ Breaking changes, major API changes"
        echo "     └─ Incompatible with previous versions"
        echo
        echo "  4) ❌ Cancel"
        echo "     └─ Exit without creating a release"
        echo
        read -p "👉 Enter your choice (1-4): " -n 1 -r
        echo
        echo
        
        case $REPLY in
            1)
                print_success "Selected: Patch release → $patch_version"
                echo "patch"
                return 0
                ;;
            2)
                print_success "Selected: Minor release → $minor_version"
                echo "minor"
                return 0
                ;;
            3)
                print_success "Selected: Major release → $major_version"
                echo "major"
                return 0
                ;;
            4)
                print_info "Release cancelled by user"
                exit 0
                ;;
            *)
                print_warning "Invalid choice '$REPLY'. Please enter 1, 2, 3, or 4."
                echo
                ;;
        esac
    done
}

# Function to get release message
get_release_message() {
    local version=$1
    local default_message="Release $version"
    
    echo
    print_info "Enter release message (or press Enter for default):"
    echo "Default: $default_message"
    read -p "Message: " release_message
    
    if [[ -z "$release_message" ]]; then
        release_message="$default_message"
    fi
    
    echo "$release_message"
}

# Function to show recent commits since last tag
show_recent_commits() {
    local latest_tag=$1
    
    echo
    print_info "Recent commits since $latest_tag:"
    echo "$(git log --oneline ${latest_tag}..HEAD | head -10)"
    
    local commit_count=$(git rev-list --count ${latest_tag}..HEAD)
    if [[ $commit_count -gt 10 ]]; then
        echo "... and $((commit_count - 10)) more commits"
    fi
    echo
}

# Function to run pre-release checks
run_pre_release_checks() {
    print_info "Running pre-release checks..."
    
    # Check if we can build
    if command -v make >/dev/null 2>&1; then
        if [[ -f "Makefile" ]]; then
            print_info "Running build check..."
            if make build >/dev/null 2>&1; then
                print_success "Build check passed"
            else
                print_error "Build check failed"
                read -p "Continue anyway? (y/N): " -n 1 -r
                echo
                if [[ ! $REPLY =~ ^[Yy]$ ]]; then
                    exit 1
                fi
            fi
        fi
    fi
    
    # Check if linter passes
    if command -v make >/dev/null 2>&1; then
        if [[ -f "Makefile" ]] && make -n lint >/dev/null 2>&1; then
            print_info "Running lint check..."
            if make lint >/dev/null 2>&1; then
                print_success "Lint check passed"
            else
                print_warning "Lint check failed"
                read -p "Continue anyway? (y/N): " -n 1 -r
                echo
                if [[ ! $REPLY =~ ^[Yy]$ ]]; then
                    exit 1
                fi
            fi
        fi
    fi
}

# Main function
main() {
    echo "🚀 DVOM Release Management"
    echo "========================="
    echo
    
    # Check prerequisites
    check_git_repo
    check_clean_working_dir
    
    # Get current tag
    local current_tag=$(get_latest_tag)
    print_info "Current latest tag: $current_tag"
    
    # Show recent commits
    if [[ "$current_tag" != "v0.0.0" ]]; then
        show_recent_commits "$current_tag"
    fi
    
    # Get user choice (includes version preview)
    local version_type=$(get_version_choice "$current_tag")
    
    # Parse current version and increment
    if ! parse_version "$current_tag"; then
        exit 1
    fi
    
    increment_version "$version_type"
    local new_version="v${MAJOR}.${MINOR}.${PATCH}"
    
    # Confirm the new version
    echo
    print_info "New version will be: $new_version"
    read -p "Proceed with this version? (Y/n): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Nn]$ ]]; then
        print_info "Release cancelled"
        exit 0
    fi
    
    # Run pre-release checks
    run_pre_release_checks
    
    # Get release message
    local release_message=$(get_release_message "$new_version")
    
    # Final confirmation
    echo
    echo "📋 Release Summary:"
    echo "   Version: $new_version"
    echo "   Type: $version_type"
    echo "   Message: $release_message"
    echo
    read -p "Create and push this release? (Y/n): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Nn]$ ]]; then
        print_info "Release cancelled"
        exit 0
    fi
    
    # Create the tag
    if create_tag "$new_version" "$release_message"; then
        echo
        print_success "🎉 Release $new_version created successfully!"
        print_info "GitHub Actions will automatically build and publish the release"
        echo
        print_info "Next steps:"
        echo "  • Check the GitHub Actions workflow"
        echo "  • Update release notes if needed"
        echo "  • Announce the release"
    else
        print_error "Failed to create release"
        exit 1
    fi
}

# Help function
show_help() {
    echo "DVOM Release Management Script"
    echo
    echo "Usage: $0 [OPTIONS]"
    echo
    echo "This script helps create new semantic version releases for DVOM."
    echo "It will:"
    echo "  • Show the current latest tag"
    echo "  • Display recent commits"
    echo "  • Allow you to choose patch/minor/major version increment"
    echo "  • Run pre-release checks (build, lint)"
    echo "  • Create and optionally push the new tag"
    echo
    echo "Options:"
    echo "  -h, --help     Show this help message"
    echo "  --dry-run      Show what would be done without making changes"
    echo
    echo "Examples:"
    echo "  $0                 # Interactive release creation"
    echo "  $0 --help          # Show this help"
    echo "  $0 --dry-run       # Preview without making changes"
    echo
    echo "Version Types:"
    echo "  Patch (X.Y.Z+1)    - Bug fixes, no API changes"
    echo "  Minor (X.Y+1.0)    - New features, backward compatible"
    echo "  Major (X+1.0.0)    - Breaking changes, major updates"
}

# Parse command line arguments
case "${1:-}" in
    -h|--help)
        show_help
        exit 0
        ;;
    --dry-run)
        print_info "DRY RUN MODE - No changes will be made"
        echo
        check_git_repo
        current_tag=$(get_latest_tag)
        print_info "Current latest tag: $current_tag"
        
        # Show recent commits
        if [[ "$current_tag" != "v0.0.0" ]]; then
            show_recent_commits "$current_tag"
        fi
        
        # Show version preview
        show_version_preview "$current_tag"
        
        print_info "This is a dry run - no tags would be created"
        print_info "Run without --dry-run to see the interactive release menu"
        exit 0
        ;;
    "")
        # No arguments, run main function
        main
        ;;
    *)
        print_error "Unknown option: $1"
        echo
        show_help
        exit 1
        ;;
esac