#!/bin/bash

# CPU Profile 分析脚本
# 用于演示如何使用 pprof 接口获取和分析 CPU profile

set -e

PPROF_HOST="localhost:6060"
PROFILE_SECONDS=30
OUTPUT_DIR="./profiles"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_header() {
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}========================================${NC}"
    echo ""
}

print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_step() {
    echo -e "\n${YELLOW}>>> $1${NC}\n"
}

# 检查程序是否运行
check_program_running() {
    print_step "Step 1: Checking if the program is running..."
    
    if curl -s "http://${PPROF_HOST}/debug/pprof/" > /dev/null 2>&1; then
        print_info "Program is running and pprof is accessible"
        return 0
    else
        print_error "Program is not running or pprof is not accessible"
        print_info "Please start the program first:"
        echo "  cd /Users/chenggang/go/src/github.com/gangcheng1030/ai_production_troubleshooting/cpu_analyze"
        echo "  go run case3_string_concat.go"
        exit 1
    fi
}

# 创建输出目录
create_output_dir() {
    print_step "Step 2: Creating output directory..."
    
    mkdir -p "${OUTPUT_DIR}"
    print_info "Output directory: ${OUTPUT_DIR}"
}

# 获取CPU profile
capture_cpu_profile() {
    print_step "Step 3: Capturing CPU profile for ${PROFILE_SECONDS} seconds..."
    
    local timestamp=$(date +%Y%m%d_%H%M%S)
    local output_file="${OUTPUT_DIR}/cpu_${timestamp}.prof"
    
    print_info "This will take ${PROFILE_SECONDS} seconds, please wait..."
    echo ""
    
    if curl -s "http://${PPROF_HOST}/debug/pprof/profile?seconds=${PROFILE_SECONDS}" -o "${output_file}"; then
        print_info "CPU profile saved to: ${output_file}"
        echo "$output_file"
    else
        print_error "Failed to capture CPU profile"
        exit 1
    fi
}

# 分析CPU profile - Top函数
analyze_top() {
    local profile_file=$1
    
    print_step "Step 4: Analyzing top CPU consumers..."
    
    print_info "Top 10 functions by flat CPU time:"
    echo ""
    go tool pprof -top -flat "${profile_file}" | head -n 20
    
    echo ""
    print_info "Top 10 functions by cumulative CPU time:"
    echo ""
    go tool pprof -top -cum "${profile_file}" | head -n 20
}

# 分析特定函数
analyze_function() {
    local profile_file=$1
    
    print_step "Step 5: Analyzing specific functions..."
    
    print_info "Analyzing badStringConcat function:"
    echo ""
    go tool pprof -list badStringConcat "${profile_file}" 2>/dev/null || print_warning "badStringConcat not found in profile"
    
    echo ""
    print_info "Analyzing goodStringBuilder function:"
    echo ""
    go tool pprof -list goodStringBuilder "${profile_file}" 2>/dev/null || print_warning "goodStringBuilder not found in profile"
}

# 生成文本报告
generate_text_report() {
    local profile_file=$1
    local report_file="${profile_file%.prof}_report.txt"
    
    print_step "Step 6: Generating text report..."
    
    {
        echo "======================================"
        echo "CPU Profile Analysis Report"
        echo "======================================"
        echo "Profile: $profile_file"
        echo "Generated: $(date)"
        echo ""
        
        echo "======================================"
        echo "Top 20 Functions (Flat)"
        echo "======================================"
        go tool pprof -top -flat "${profile_file}" | head -n 25
        
        echo ""
        echo "======================================"
        echo "Top 20 Functions (Cumulative)"
        echo "======================================"
        go tool pprof -top -cum "${profile_file}" | head -n 25
        
        echo ""
        echo "======================================"
        echo "badStringConcat Details"
        echo "======================================"
        go tool pprof -list badStringConcat "${profile_file}" 2>/dev/null || echo "Not found in profile"
        
        echo ""
        echo "======================================"
        echo "goodStringBuilder Details"
        echo "======================================"
        go tool pprof -list goodStringBuilder "${profile_file}" 2>/dev/null || echo "Not found in profile"
        
    } > "${report_file}"
    
    print_info "Text report saved to: ${report_file}"
}

# 生成火焰图
generate_flamegraph() {
    local profile_file=$1
    
    print_step "Step 7: Opening interactive web UI..."
    
    print_info "Starting pprof web UI on http://localhost:8080"
    print_info "You can view:"
    print_info "  - Flamegraph: http://localhost:8080/ui/flamegraph"
    print_info "  - Top: http://localhost:8080/ui/top"
    print_info "  - Graph: http://localhost:8080/ui/"
    echo ""
    print_warning "Press Ctrl+C to stop the web server"
    echo ""
    
    go tool pprof -http=:8080 "${profile_file}"
}

# 比较两个profile
compare_profiles() {
    print_step "Comparing profiles..."
    
    local profiles=(${OUTPUT_DIR}/cpu_*.prof)
    
    if [ ${#profiles[@]} -lt 2 ]; then
        print_warning "Need at least 2 profiles to compare"
        return
    fi
    
    local base_profile="${profiles[-2]}"
    local new_profile="${profiles[-1]}"
    
    print_info "Comparing:"
    print_info "  Base: ${base_profile}"
    print_info "  New:  ${new_profile}"
    echo ""
    
    go tool pprof -base="${base_profile}" -top "${new_profile}" | head -n 20
}

# 查看当前goroutine状态
view_goroutines() {
    print_step "Viewing current goroutine status..."
    
    print_info "Goroutine count:"
    curl -s "http://${PPROF_HOST}/debug/pprof/goroutine?debug=1" | grep "goroutine profile:" || true
    
    echo ""
    print_info "Top goroutine stacks:"
    curl -s "http://${PPROF_HOST}/debug/pprof/goroutine?debug=1" | head -n 50
}

# 主菜单
show_menu() {
    echo ""
    print_header "CPU Profile Analysis Menu"
    echo "1) Capture and analyze CPU profile (quick)"
    echo "2) Capture CPU profile and open web UI"
    echo "3) Capture profile with custom duration"
    echo "4) Compare latest two profiles"
    echo "5) View goroutine status"
    echo "6) List all captured profiles"
    echo "7) Open existing profile in web UI"
    echo "8) Full analysis (all steps)"
    echo "0) Exit"
    echo ""
}

# 快速分析
quick_analysis() {
    check_program_running
    create_output_dir
    
    local profile_file=$(capture_cpu_profile)
    
    analyze_top "${profile_file}"
    analyze_function "${profile_file}"
    generate_text_report "${profile_file}"
    
    echo ""
    print_info "Analysis complete!"
    print_info "To view in web UI, run: go tool pprof -http=:8080 ${profile_file}"
}

# 列出所有profiles
list_profiles() {
    print_step "Listing all captured profiles..."
    
    if [ -d "${OUTPUT_DIR}" ]; then
        ls -lh "${OUTPUT_DIR}"/*.prof 2>/dev/null || print_warning "No profiles found"
    else
        print_warning "No profiles directory found"
    fi
}

# 打开已有的profile
open_existing_profile() {
    list_profiles
    echo ""
    read -p "Enter profile filename (or path): " profile_file
    
    if [ -f "${profile_file}" ]; then
        generate_flamegraph "${profile_file}"
    elif [ -f "${OUTPUT_DIR}/${profile_file}" ]; then
        generate_flamegraph "${OUTPUT_DIR}/${profile_file}"
    else
        print_error "Profile file not found: ${profile_file}"
    fi
}

# 自定义时长
custom_duration_analysis() {
    check_program_running
    create_output_dir
    
    echo ""
    read -p "Enter duration in seconds (default: 30): " duration
    duration=${duration:-30}
    
    PROFILE_SECONDS=$duration
    
    local profile_file=$(capture_cpu_profile)
    
    echo ""
    read -p "Open in web UI? (y/n): " open_ui
    if [ "$open_ui" == "y" ]; then
        generate_flamegraph "${profile_file}"
    else
        analyze_top "${profile_file}"
        generate_text_report "${profile_file}"
    fi
}

# 完整分析
full_analysis() {
    check_program_running
    create_output_dir
    view_goroutines
    
    local profile_file=$(capture_cpu_profile)
    
    analyze_top "${profile_file}"
    analyze_function "${profile_file}"
    generate_text_report "${profile_file}"
    
    echo ""
    read -p "Open in web UI? (y/n): " open_ui
    if [ "$open_ui" == "y" ]; then
        generate_flamegraph "${profile_file}"
    fi
}

# 主程序
main() {
    print_header "CPU Profile Analysis Tool for String Concatenation"
    
    # 如果有命令行参数，执行快速分析
    if [ "$1" == "quick" ]; then
        quick_analysis
        exit 0
    fi
    
    if [ "$1" == "web" ]; then
        check_program_running
        create_output_dir
        local profile_file=$(capture_cpu_profile)
        generate_flamegraph "${profile_file}"
        exit 0
    fi
    
    # 交互式菜单
    while true; do
        show_menu
        read -p "Choose an option: " choice
        
        case $choice in
            1)
                quick_analysis
                ;;
            2)
                check_program_running
                create_output_dir
                local profile_file=$(capture_cpu_profile)
                generate_flamegraph "${profile_file}"
                ;;
            3)
                custom_duration_analysis
                ;;
            4)
                compare_profiles
                ;;
            5)
                check_program_running
                view_goroutines
                ;;
            6)
                list_profiles
                ;;
            7)
                open_existing_profile
                ;;
            8)
                full_analysis
                ;;
            0)
                print_info "Goodbye!"
                exit 0
                ;;
            *)
                print_error "Invalid option"
                ;;
        esac
        
        echo ""
        read -p "Press Enter to continue..."
    done
}

# 运行主程序
main "$@"

