//go:build ignore

package main

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// ============================================================
// 可配置变量（环境变量优先）
// ============================================================
//
// 约定：你从 dwg-go 根目录执行：
//
//	go run ./dwg_service/build_libredwg/build_libredwg.go
//
// SRCDIR 语义：生成的 cgo 文件会被 dwg_service 包编译，
// 所以 cgo 文件里用 ${SRCDIR}/libs/... 是正确的（SRCDIR=dwg_service目录）。
var (
	// 构建缓存根目录（不提交）
	// 默认: dwg_service/build
	BuildRoot = getEnvOrDefault("DWG_BUILD_ROOT", filepath.Join("dwg_service", "build"))

	// staging install 根目录（configure/make install 写这里）
	// 默认: dwg_service/build/_install
	InstallRoot = getEnvOrDefault("DWG_INSTALL_ROOT", filepath.Join("dwg_service", "build", "_install"))

	// 最终产物固定根目录（可提交或用于打包）
	// 默认: dwg_service/libs/libredwg
	FinalRoot = getEnvOrDefault("DWG_FINAL_ROOT", filepath.Join("dwg_service", "libs", "libredwg"))

	// libredwg 源码目录（在 BuildRoot 下）
	// 默认: <BuildRoot>/libredwg
	SourceDirName = getEnvOrDefault("DWG_SRC_DIRNAME", "libredwg")

	// LibreDWG 仓库地址
	RepoURL = getEnvOrDefault("DWG_REPO_URL", "https://github.com/LibreDWG/libredwg.git")

	// 目标版本：可以是 tag/branch/commit
	// 默认: master
	RepoRef = getEnvOrDefault("DWG_REPO_REF", "master")

	// 是否强制重新拉取/清理 build 目录
	Clean = strings.ToLower(os.Getenv("DWG_CLEAN")) == "1" || strings.ToLower(os.Getenv("DWG_CLEAN")) == "true"

	// 生成 cgo 文件输出目录（就是 dwg_service 包目录）
	// 默认: dwg_service
	CGOOutDir = getEnvOrDefault("DWG_CGO_OUT_DIR", "dwg_service")

	// 生成的 cgo 文件 package 名
	// dwg_service 本身是 main 包的话，这里也必须是 main
	// 默认: main
	CGOPackage = getEnvOrDefault("DWG_CGO_PACKAGE", "main")

	// 目标输出目录命名：<os>-<arch>
	// 例如 linux-amd64, windows-amd64, darwin-arm64
	// 默认: runtime.GOOS-runtime.GOARCH
	TargetTriple = os.Getenv("DWG_TARGET_TRIPLE")

	// Windows 下执行构建命令时使用的 bash（可选）
	// 默认: bash（要求在 PATH）
	WindowsBash = getEnvOrDefault("DWG_WINDOWS_BASH", "bash")

	// pkg-config 的候选包名（不同发行版可能不一致）
	// 你可以通过环境变量 DWG_PKG_NAMES 覆盖（逗号分隔）
	PkgNameCandidates = getEnvOrDefault("DWG_PKG_NAMES", "libredwg,redwg,libreDWG,libredwg-0,libredwg-0.1")
)

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// ============================================================
// 构建上下文
// ============================================================

type buildCtx struct {
	goos   string
	goarch string

	target string

	// 目录
	buildRoot  string
	srcRoot    string
	srcDir     string
	buildDir   string
	installDir string
	finalRoot  string

	// final per target
	finalInclude string
	finalLibDir  string
	finalLibA    string

	// wd
	wd string
}

func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		fmt.Printf("Cannot get current directory: %v\n", err)
		os.Exit(1)
	}
	return wd
}

func newBuildCtx() *buildCtx {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	target := TargetTriple
	if target == "" {
		target = fmt.Sprintf("%s-%s", goos, goarch)
	}

	wd := mustGetwd()

	buildRoot := filepath.Clean(BuildRoot)
	srcRoot := filepath.Join(buildRoot, "_src")
	srcDir := filepath.Join(srcRoot, SourceDirName)

	// 每个 target 必须隔离 build 目录，避免混编
	buildDir := filepath.Join(buildRoot, fmt.Sprintf("_build_%s", target))
	installDir := filepath.Join(filepath.Clean(InstallRoot), target)

	finalRoot := filepath.Clean(FinalRoot)
	finalInclude := filepath.Join(finalRoot, "include")
	finalLibDir := filepath.Join(finalRoot, target, "lib")

	return &buildCtx{
		goos:         goos,
		goarch:       goarch,
		target:       target,
		buildRoot:    buildRoot,
		srcRoot:      srcRoot,
		srcDir:       srcDir,
		buildDir:     buildDir,
		installDir:   installDir,
		finalRoot:    finalRoot,
		finalInclude: finalInclude,
		finalLibDir:  finalLibDir,
		wd:           wd,
	}
}

// ============================================================
// 工具/FS helpers
// ============================================================

func mustMkdirAll(path string, perm os.FileMode) {
	if err := os.MkdirAll(path, perm); err != nil {
		fmt.Printf("Could not create directory %s: %v\n", path, err)
		os.Exit(1)
	}
}

func mustRemoveAll(path string) {
	_ = os.RemoveAll(path)
}

func mustCheckTool(tool string) {
	if _, err := exec.LookPath(tool); err != nil {
		fmt.Printf("Cannot find required tool %q in PATH: %v\n", tool, err)
		os.Exit(1)
	}
}

func copyFile(src, dst string, perm fs.FileMode) {
	in, err := os.Open(src)
	if err != nil {
		fmt.Printf("Error opening %s: %v\n", src, err)
		os.Exit(1)
	}
	defer in.Close()

	mustMkdirAll(filepath.Dir(dst), 0750)

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, perm)
	if err != nil {
		fmt.Printf("Error creating %s: %v\n", dst, err)
		os.Exit(1)
	}
	defer out.Close()

	if _, err := out.ReadFrom(in); err != nil {
		fmt.Printf("Error copying %s -> %s: %v\n", src, dst, err)
		os.Exit(1)
	}
}

func copyDir(srcDir, dstDir string) {
	err := filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(srcDir, path)
		dst := filepath.Join(dstDir, rel)

		if d.IsDir() {
			return os.MkdirAll(dst, 0750)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		copyFile(path, dst, info.Mode().Perm())
		return nil
	})
	if err != nil {
		fmt.Printf("Error copying dir %s -> %s: %v\n", srcDir, dstDir, err)
		os.Exit(1)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ============================================================
// 命令执行：Unix/Windows(MSYS2 bash)
// ============================================================

func runCmd(dir, name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("Error running command: %s %s (dir=%s): %v\n", name, strings.Join(args, " "), dir, err)
		os.Exit(1)
	}
}

func outputCmd(dir, name string, args ...string) []byte {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		fmt.Printf("Error running command: %s %s (dir=%s): %v\n", name, strings.Join(args, " "), dir, err)
		os.Exit(1)
	}
	return out
}

// Windows 下用 bash -lc 运行一串命令（在 MSYS2/MinGW 环境里更稳）
func runBash(dir string, script string, extraEnv map[string]string) {
	bash := WindowsBash
	// bash -lc "cmds"
	cmd := exec.Command(bash, "-lc", script)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if len(extraEnv) > 0 {
		env := os.Environ()
		for k, v := range extraEnv {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		cmd.Env = env
	}

	if err := cmd.Run(); err != nil {
		fmt.Printf("Error running bash script (dir=%s): %v\nScript:\n%s\n", dir, err, script)
		os.Exit(1)
	}
}

func outputBash(dir string, script string, extraEnv map[string]string) []byte {
	bash := WindowsBash
	cmd := exec.Command(bash, "-lc", script)
	cmd.Dir = dir

	if len(extraEnv) > 0 {
		env := os.Environ()
		for k, v := range extraEnv {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		cmd.Env = env
	}

	out, err := cmd.Output()
	if err != nil {
		fmt.Printf("Error running bash output script (dir=%s): %v\nScript:\n%s\n", dir, err, script)
		os.Exit(1)
	}
	return out
}

// ============================================================
// 源码准备（git clone / checkout ref）
// ============================================================

func ensureSource(ctx *buildCtx) {
	mustMkdirAll(ctx.srcRoot, 0750)

	if Clean {
		fmt.Println("DWG_CLEAN enabled: removing build cache")
		mustRemoveAll(ctx.buildRoot)
		mustMkdirAll(ctx.srcRoot, 0750)
	}

	if !fileExists(ctx.srcDir) {
		fmt.Println("Cloning libredwg repository...")
		runCmd(ctx.srcRoot, "git", "clone", RepoURL, SourceDirName)
	} else {
		fmt.Println("Found existing libredwg source")
		runCmd(ctx.srcDir, "git", "fetch", "--all", "--tags")
	}

	// checkout ref
	runCmd(ctx.srcDir, "git", "checkout", RepoRef)
	runCmd(ctx.srcDir, "git", "submodule", "update", "--init", "--recursive")
}

// ============================================================
// 构建（Autotools 典型流程）
// ============================================================

func ensureAutotoolsReady(ctx *buildCtx) {
	// 典型文件：configure
	if fileExists(filepath.Join(ctx.srcDir, "configure")) {
		return
	}

	fmt.Println("No ./configure found, trying autoreconf -fi ...")

	if ctx.goos == "windows" {
		runBash(ctx.srcDir, "autoreconf -fi", nil)
	} else {
		mustCheckTool("autoreconf")
		runCmd(ctx.srcDir, "autoreconf", "-fi")
	}

	if !fileExists(filepath.Join(ctx.srcDir, "configure")) {
		fmt.Println("Still no ./configure after autoreconf. Your libredwg source may not use autotools.")
		fmt.Println("If it uses meson/cmake, adjust build script accordingly.")
		os.Exit(1)
	}
}

func configureAndBuild(ctx *buildCtx) {
	// 清理 build/install 目录（target 隔离）
	mustRemoveAll(ctx.buildDir)
	mustRemoveAll(ctx.installDir)
	mustMkdirAll(ctx.buildDir, 0750)
	mustMkdirAll(ctx.installDir, 0750)

	// configure 参数：尽量锁死静态库
	// 注：不同版本支持的选项可能略有差异，失败时你需要根据 configure --help 微调
	prefix := filepath.ToSlash(mustAbs(ctx.installDir))

	configArgs := []string{
		fmt.Sprintf("--prefix=%s", prefix),
		"--disable-shared",
		"--enable-static",
	}

	fmt.Printf("Configuring libredwg (prefix=%s)\n", prefix)

	if ctx.goos == "windows" {
		// Windows：建议在 MSYS2/MinGW shell 里跑（bash -lc）
		// buildDir 内跑 configure（源代码目录/相对路径）
		script := strings.Join([]string{
			`set -e`,
			fmt.Sprintf(`cd "%s"`, msysPath(ctx.buildDir)),
			fmt.Sprintf(`"%s/configure" %s`, msysPath(ctx.srcDir), strings.Join(configArgs, " ")),
			`make -j`,
			`make install`,
		}, "\n")

		runBash(ctx.buildDir, script, nil)
	} else {
		// Unix：在 buildDir 内 out-of-tree 配置
		runCmd(ctx.buildDir, filepath.Join(ctx.srcDir, "configure"), configArgs...)
		runCmd(ctx.buildDir, "make", "-j")
		runCmd(ctx.buildDir, "make", "install")
	}
}

func mustAbs(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		fmt.Printf("Error resolving abs path %s: %v\n", p, err)
		os.Exit(1)
	}
	return abs
}

// Windows bash 环境下，尽量转换为 /c/... 形式（粗略但好用）
// 依赖 bash 的路径解析能力：/c/xxx 或 C:/xxx 均可能可用
func msysPath(path string) string {
	p := filepath.ToSlash(mustAbs(path))
	// C:/xx -> /c/xx
	if len(p) >= 3 && p[1] == ':' && p[2] == '/' {
		drive := strings.ToLower(string(p[0]))
		return "/" + drive + p[2:]
	}
	return p
}

// ============================================================
// 同步产物到 FinalRoot（dwg_service/libs/libredwg/...）
// ============================================================

func syncToFinal(ctx *buildCtx) {
	// include：从 installDir/include -> finalRoot/include
	installInclude := filepath.Join(ctx.installDir, "include")
	if !fileExists(installInclude) {
		// 有些项目会装到 include/libredwg 或 include/...
		// 这里保守：如果 include 不存在就报错，让你看 installDir 结构
		fmt.Printf("install include dir not found: %s\n", installInclude)
		os.Exit(1)
	}

	// lib：可能在 lib 或 lib64
	installLibDir := filepath.Join(ctx.installDir, "lib")
	if !fileExists(installLibDir) {
		alt := filepath.Join(ctx.installDir, "lib64")
		if fileExists(alt) {
			installLibDir = alt
		} else {
			fmt.Printf("install lib dir not found: %s (or lib64)\n", filepath.Join(ctx.installDir, "lib"))
			os.Exit(1)
		}
	}

	// 找 libredwg.a（名称可能为 libredwg.a 或 libredwg-*.a）
	libA := findStaticLib(installLibDir)
	if libA == "" {
		fmt.Printf("No static lib found under %s\n", installLibDir)
		os.Exit(1)
	}

	// 目标 final 目录
	mustMkdirAll(ctx.finalRoot, 0750)
	mustMkdirAll(ctx.finalInclude, 0750)
	mustMkdirAll(ctx.finalLibDir, 0750)

	// 同步 include（覆盖）
	mustRemoveAll(ctx.finalInclude)
	copyDir(installInclude, ctx.finalInclude)

	// 同步 lib（仅拷贝静态库本体；你也可以改成拷贝整个 lib 目录）
	dstA := filepath.Join(ctx.finalLibDir, filepath.Base(libA))
	copyFile(libA, dstA, 0644)

	ctx.finalLibA = dstA

	fmt.Printf("Synced include -> %s\n", ctx.finalInclude)
	fmt.Printf("Synced static lib -> %s\n", ctx.finalLibA)
}

func findStaticLib(libDir string) string {
	entries, err := os.ReadDir(libDir)
	if err != nil {
		fmt.Printf("Cannot read dir %s: %v\n", libDir, err)
		os.Exit(1)
	}

	var candidates []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if strings.HasSuffix(n, ".a") && strings.HasPrefix(n, "lib") && strings.Contains(strings.ToLower(n), "dwg") {
			candidates = append(candidates, filepath.Join(libDir, n))
		}
	}
	// 如果没匹配到含 dwg 的，退一步找 libredwg.a
	if len(candidates) == 0 {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			n := e.Name()
			if n == "libredwg.a" {
				candidates = append(candidates, filepath.Join(libDir, n))
			}
		}
	}
	if len(candidates) == 0 {
		return ""
	}
	sort.Strings(candidates)
	// 取最“像”libredwg 的
	for _, c := range candidates {
		if filepath.Base(c) == "libredwg.a" {
			return c
		}
	}
	return candidates[0]
}

// ============================================================
// pkg-config flags（候选名探测 + fallback）
// ============================================================

func pkgNames() []string {
	parts := strings.Split(PkgNameCandidates, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func tryPkgConfig(ctx *buildCtx) (cflags string, libsStatic string, ok bool) {
	names := pkgNames()

	// pkg-config 环境：指向 installDir 的 pkgconfig
	pcPaths := []string{
		filepath.ToSlash(filepath.Join(mustAbs(ctx.installDir), "lib", "pkgconfig")),
		filepath.ToSlash(filepath.Join(mustAbs(ctx.installDir), "lib64", "pkgconfig")),
		filepath.ToSlash(filepath.Join(mustAbs(ctx.installDir), "share", "pkgconfig")),
	}
	extraEnv := map[string]string{
		"PKG_CONFIG_PATH": strings.Join(pcPaths, string(os.PathListSeparator)),
	}

	if ctx.goos == "windows" {
		// Windows：用 bash 调 pkg-config
		for _, n := range names {
			// cflags
			cf := strings.TrimSpace(string(outputBash(ctx.wd, fmt.Sprintf(`pkg-config --cflags %s 2>/dev/null || true`, n), extraEnv)))
			ls := strings.TrimSpace(string(outputBash(ctx.wd, fmt.Sprintf(`pkg-config --libs --static %s 2>/dev/null || true`, n), extraEnv)))
			if cf != "" || ls != "" {
				return cf, ls, true
			}
		}
		return "", "", false
	}

	// Unix：直接调用 pkg-config
	if _, err := exec.LookPath("pkg-config"); err != nil {
		return "", "", false
	}

	for _, n := range names {
		cmdCf := exec.Command("pkg-config", "--cflags", n)
		cmdLs := exec.Command("pkg-config", "--libs", "--static", n)
		cmdCf.Env = append(os.Environ(), fmt.Sprintf("PKG_CONFIG_PATH=%s", extraEnv["PKG_CONFIG_PATH"]))
		cmdLs.Env = cmdCf.Env

		cfOut, _ := cmdCf.Output()
		lsOut, _ := cmdLs.Output()

		cf := strings.TrimSpace(string(cfOut))
		ls := strings.TrimSpace(string(lsOut))
		if cf != "" || ls != "" {
			return cf, ls, true
		}
	}
	return "", "", false
}

// rewrite 安装路径到 ${SRCDIR}/libs/libredwg 结构
func rewriteFlagsForCgo(ctx *buildCtx, s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}

	// 生成的 cgo 文件在 dwg_service/ 下：${SRCDIR} = <repo>/dwg_service
	// FinalRoot 默认是 dwg_service/libs/libredwg（相对 repo）
	// 我们把 finalRoot 变成 ${SRCDIR}/libs/libredwg
	finalRootRel := "libs/libredwg"
	finalRootCgo := "${SRCDIR}/" + filepath.ToSlash(finalRootRel)

	installAbs := filepath.ToSlash(filepath.Clean(mustAbs(ctx.installDir)))
	finalAbs := filepath.ToSlash(filepath.Clean(mustAbs(filepath.Join("dwg_service", "libs", "libredwg"))))

	// 统一斜杠
	ss := strings.ReplaceAll(s, "\\", "/")

	// 先把 install 前缀替换掉，再把可能的绝对 final 替换为 SRCDIR 相对（防止有人手动改过）
	ss = strings.ReplaceAll(ss, installAbs, finalRootCgo)
	ss = strings.ReplaceAll(ss, finalAbs, finalRootCgo)

	// 压缩空白
	return strings.TrimSpace(ss)
}

// token 去重
func dedupBySpace(s string) string {
	fields := strings.Fields(s)
	seen := make(map[string]bool, len(fields))
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if !seen[f] {
			seen[f] = true
			out = append(out, f)
		}
	}
	return strings.Join(out, " ")
}

// ============================================================
// 生成 cgo 文件（dwg_service/cgo_<os>_<arch>.go）
// ============================================================

func generateCgo(ctx *buildCtx) {
	mustMkdirAll(CGOOutDir, 0750)

	// 文件名
	name := fmt.Sprintf("cgo_%s_%s.go", ctx.goos, ctx.goarch)
	outPath := filepath.Join(CGOOutDir, name)

	f, err := os.Create(outPath)
	if err != nil {
		fmt.Printf("Error creating %s: %v\n", outPath, err)
		os.Exit(1)
	}
	defer f.Close()

	// build tag
	fmt.Fprintf(f, "//go:build %s && %s\n\n", ctx.goos, ctx.goarch)
	fmt.Fprintf(f, "package %s\n\n", CGOPackage)

	// include & lib 的最终路径（相对 SRCDIR）
	inc := "${SRCDIR}/libs/libredwg/include"
	libDir := fmt.Sprintf("${SRCDIR}/libs/libredwg/%s/lib", ctx.target)
	// 静态库路径（直接指定 .a 更稳）
	libA := fmt.Sprintf("%s/%s", libDir, filepath.Base(ctx.finalLibA))

	// 取 pkg-config flags（尽量完整），并 rewrite 到 SRCDIR 路径
	cflags, ldflags, ok := tryPkgConfig(ctx)
	cflags = rewriteFlagsForCgo(ctx, cflags)
	ldflags = rewriteFlagsForCgo(ctx, ldflags)

	// CPPFLAGS：我们自己的 -I 永远置前
	// 另外补充 largefile 宏（Linux/Windows 常见需要，macOS 通常不需要，但加了也没大碍）
	cpp := strings.TrimSpace(strings.Join([]string{
		"-I" + inc,
		"-D_LARGEFILE_SOURCE",
		"-D_LARGEFILE64_SOURCE",
		"-D_FILE_OFFSET_BITS=64",
		cflags,
	}, " "))

	// LDFLAGS：优先用“直接静态库路径 + pkg-config static libs”
	// 如果 pkg-config 不可用，则 fallback：直接用 .a + 常见依赖
	ld := ""
	if ok && strings.TrimSpace(ldflags) != "" {
		ld = strings.TrimSpace(strings.Join([]string{libA, ldflags}, " "))
	} else {
		// fallback：最保守的一组（你后续如遇缺符号再补）
		// -lm (math), -lz (zlib), -liconv (some platforms)
		ld = strings.TrimSpace(strings.Join([]string{libA, "-lm", "-lz", "-liconv"}, " "))
	}

	// token 去重 + 清理多余空白
	cpp = dedupBySpace(cpp)
	ld = dedupBySpace(ld)

	fmt.Fprintf(f, "// #cgo CPPFLAGS: %s\n", cpp)
	fmt.Fprintf(f, "// #cgo LDFLAGS: %s\n", ld)
	fmt.Fprintln(f, `import "C"`)

	fmt.Printf("Generated cgo file: %s\n", outPath)
}

// ============================================================
// 主流程
// ============================================================

func mustCheckEnv() {
	fmt.Printf("Building libredwg for %s/%s (target=%s)\n", runtime.GOOS, runtime.GOARCH, TargetTripleOrDefault())
}

func TargetTripleOrDefault() string {
	if TargetTriple != "" {
		return TargetTriple
	}
	return fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
}

func mustCheckToolsForPlatform(ctx *buildCtx) {
	mustCheckTool("git")

	if ctx.goos == "windows" {
		// Windows：我们依赖 bash 环境（Git-Bash 或 MSYS2）
		mustCheckTool(WindowsBash)
		// 这些工具最好也在 bash 环境里存在（MSYS2）
		// 在这里不逐个 mustCheckTool（因为可能只在 bash 内部），后续命令失败会直接报错。
		return
	}

	// Unix：本机工具链
	mustCheckTool("make")
	mustCheckTool("cc") // gcc/clang 通常映射为 cc
	// 如果没有 autoreconf，脚本会提示
	if _, err := exec.LookPath("autoreconf"); err != nil {
		fmt.Println("Warning: autoreconf not found. If your source lacks ./configure, build will fail.")
	}
}

func prepareDirs(ctx *buildCtx) {
	mustMkdirAll(ctx.buildRoot, 0750)
	mustMkdirAll(ctx.srcRoot, 0750)
	mustMkdirAll(ctx.installDir, 0750)
	mustMkdirAll(ctx.finalRoot, 0750)
}

func main() {
	mustCheckEnv()

	ctx := newBuildCtx()
	prepareDirs(ctx)
	mustCheckToolsForPlatform(ctx)

	ensureSource(ctx)
	ensureAutotoolsReady(ctx)
	configureAndBuild(ctx)
	syncToFinal(ctx)
	generateCgo(ctx)

	fmt.Println("Done.")
	fmt.Printf("Final artifacts:\n  include: %s\n  lib:     %s\n", ctx.finalInclude, ctx.finalLibA)
}
