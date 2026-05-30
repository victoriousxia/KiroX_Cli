package browser

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math/rand"
	"sort"
)

// ──────────────────── Chrome 多版本支持 ────────────────────

type chromeVersion struct {
	Version string
	SecUA   string
}

// genChromeVersion 动态生成 Chrome 版本信息（仅使用近期版本）
func genChromeVersion() chromeVersion {
	// 只保留最近 2 个大版本，模拟真实用户的自动更新行为
	versions := []string{"136", "137"}
	v := versions[rand.Intn(len(versions))]

	greaseBrands := []string{"Not_A Brand", "Not(A:Brand", "Not-A.Brand", "Not)A;Brand", "Not/A)Brand", "Not A;Brand", "Not?A_Brand"}
	greaseBrand := greaseBrands[rand.Intn(len(greaseBrands))]
	greaseVer := fmt.Sprintf("%d", 8+rand.Intn(92)) // 8~99

	secUA := fmt.Sprintf(`"%s";v="%s", "Chromium";v="%s", "Google Chrome";v="%s"`, greaseBrand, greaseVer, v, v)

	// 生成真实的小版本号 (major.minor.build.patch)
	minor := rand.Intn(2) // 0-1
	build := 6700 + rand.Intn(300)
	patch := rand.Intn(200)
	fullVer := fmt.Sprintf("%s.%d.%d.%d", v, minor, build, patch)

	return chromeVersion{
		Version: fullVer,
		SecUA:   secUA,
	}
}

// ──────────────────── lsUbid 前缀池 ────────────────────

var lsubidPrefixes = []string{"X10", "X19", "X42", "X55", "X73", "X81", "X96"}

// ──────────────────── WebGL 扩展 ────────────────────

var webglExtCore = []string{
	"ANGLE_instanced_arrays", "EXT_blend_minmax", "EXT_color_buffer_half_float",
	"EXT_float_blend", "EXT_frag_depth", "EXT_shader_texture_lod",
	"EXT_texture_filter_anisotropic", "EXT_sRGB", "KHR_parallel_shader_compile",
	"OES_element_index_uint", "OES_fbo_render_mipmap", "OES_standard_derivatives",
	"OES_texture_float", "OES_texture_float_linear", "OES_texture_half_float",
	"OES_texture_half_float_linear", "OES_vertex_array_object",
	"WEBGL_color_buffer_float", "WEBGL_compressed_texture_s3tc",
	"WEBGL_compressed_texture_s3tc_srgb", "WEBGL_debug_renderer_info",
	"WEBGL_debug_shaders", "WEBGL_depth_texture", "WEBGL_draw_buffers",
	"WEBGL_lose_context", "WEBGL_multi_draw",
}

var webglExtOptional = []string{
	"EXT_disjoint_timer_query", "EXT_texture_compression_bptc",
	"EXT_texture_compression_rgtc", "WEBGL_compressed_texture_astc",
	"WEBGL_compressed_texture_etc", "OES_draw_buffers_indexed",
	"EXT_color_buffer_float",
}

// ──────────────────── 插件 (Chrome 固有) ────────────────────

var pluginsPool = []map[string]string{
	{"name": "PDF Viewer", "filename": "internal-pdf-viewer", "description": "Portable Document Format"},
	{"name": "Chrome PDF Viewer", "filename": "internal-pdf-viewer", "description": "Portable Document Format"},
	{"name": "Chromium PDF Viewer", "filename": "internal-pdf-viewer", "description": "Portable Document Format"},
	{"name": "Microsoft Edge PDF Viewer", "filename": "internal-pdf-viewer", "description": "Portable Document Format"},
	{"name": "WebKit built-in PDF", "filename": "internal-pdf-viewer", "description": "Portable Document Format"},
}

// ──────────────────── 数据结构 ────────────────────

// ScreenInfo 屏幕信息
type ScreenInfo struct {
	Width, Height, AvailWidth, AvailHeight, ColorDepth int
}

// BrowserIdentity 浏览器身份
type BrowserIdentity struct {
	ChromeVer           string
	UA                  string
	SecUA               string
	GPUVendor           string
	GPUModel            string
	WebGLExts           []string
	CanvasHash          int32
	HistogramBase       [256]int
	MathTan             string
	MathSin             string
	MathCos             string
	Plugins             []map[string]string
	Screen              ScreenInfo
	DeviceMemory        int
	HardwareConcurrency int
	Platform            string
	TimeZone            int
	LsubidPrefixSignin  string
	LsubidPrefixProfile string
	WebpackHash         string
}

// ──────────────────── 算法: Math 精度生成 ────────────────────
// 规律: Math.tan/sin/cos(-1e300) 在不同硬件上仅末位 1-2 位有差异
// tan 基准: "-1.4214488238747245"  末位 3~7
// sin 基准: "0.8178819121159085"   末位 3~7
// cos 有两个家族:
//   家族A: "-0.5753861119575491"   末位 89~93
//   家族B: "-0.5765775004286854"   末位 53~55

func genMath() (tan, sin, cos string) {
	// 真实浏览器 Math.tan/sin/cos(-1e300) 仅末位 1-2 位有微小差异
	tanEnds := []string{"45", "46", "47", "48", "43", "44"}
	sinEnds := []string{"85", "86", "84", "87", "83"}
	tan = "-1.42144882387472" + tanEnds[rand.Intn(len(tanEnds))]
	sin = "0.81788191211590" + sinEnds[rand.Intn(len(sinEnds))]

	// cos 有两个家族, 家族A 更常见 (~70%)
	if rand.Intn(10) < 7 {
		cosAEnds := []string{"91", "90", "89", "92", "93"}
		cos = "-0.57538611195754" + cosAEnds[rand.Intn(len(cosAEnds))]
	} else {
		cosBEnds := []string{"54", "53", "55", "52"}
		cos = "-0.57657750042868" + cosBEnds[rand.Intn(len(cosBEnds))]
	}
	return
}

// ──────────────────── 算法: Canvas Histogram 模拟 ────────────────────
// 分析 collect_histogram.html 的渲染逻辑 (150×60 canvas, 36000 RGBA 样本):
//
// 1. 大量透明背景 → bins[0] 极大 (R=0,G=0,B=0,A=0 → 每个透明像素贡献 4 个 0)
// 2. 绘制区域 alpha=255 → bins[255] 极大
// 3. #f60 = RGB(255,102,0) 矩形 → bins[102] 出现 spike (~500-700)
// 4. multiply/difference 混合模式 + 圆形 → 产生 ~153 附近的 spike (~400-700)
// 5. 文字抗锯齿 + 渐变 + 曲线 → 中间 bins 分散在 4-120

func generateCanvasData() (int32, [256]int) {
	var bins [256]int
	const totalSamples = 36000 // 150×60×4(RGBA)

	// ── 主峰: 背景透明区 + Alpha 通道 ──
	bins[0] = 5000 + rand.Intn(10001)   // 5000~15000
	bins[255] = 6000 + rand.Intn(10001) // 6000~16000

	// ── 次要 spike: 来自特定颜色值 ──
	// #f60 矩形的 G 通道 = 102
	spike1Pos := 100 + rand.Intn(6) // 100~105, 围绕 102
	bins[spike1Pos] = 500 + rand.Intn(200)

	// multiply 混合产生的值, 围绕 153
	spike2Pos := 150 + rand.Intn(8) // 150~157, 围绕 153
	bins[spike2Pos] = 400 + rand.Intn(300)

	// ── 计算已分配的样本数 ──
	assigned := bins[0] + bins[255] + bins[spike1Pos] + bins[spike2Pos]
	remaining := totalSamples - assigned

	// ── 特征颜色区 (中等值, 来自绘图操作) ──
	// 这些是 circle/gradient/text 渲染常产生的值范围
	featureBins := []struct {
		lo, hi   int // bin 范围
		avgCount int // 每个 bin 平均计数
	}{
		{1, 30, 30},    // 深色区 (暗部抗锯齿)
		{31, 70, 20},   // 中暗区 (曲线/渐变)
		{71, 99, 25},   // 中间区 (混合色)
		{106, 149, 15}, // 中高区 (文字/渐变过渡)
		{158, 200, 18}, // 高亮区 (圆形着色)
		{201, 254, 25}, // 亮区 (接近白色)
	}

	for _, fb := range featureBins {
		for i := fb.lo; i <= fb.hi; i++ {
			if remaining <= 0 {
				break
			}
			// 正态分布风格: 中心值多，两侧少
			center := (fb.lo + fb.hi) / 2
			dist := abs(i - center)
			maxDist := (fb.hi - fb.lo) / 2
			if maxDist == 0 {
				maxDist = 1
			}
			// 衰减因子
			scale := float64(maxDist-dist) / float64(maxDist)
			if scale < 0.2 {
				scale = 0.2
			}
			base := int(float64(fb.avgCount) * scale)
			v := max(2, base+rand.Intn(max(1, base/2+1))-base/4)
			if v > remaining {
				v = remaining
			}
			bins[i] = v
			remaining -= v
		}
	}

	// ── 未分配的 bins 补充少量噪声 ──
	for i := 1; i < 255; i++ {
		if bins[i] == 0 && remaining > 0 {
			v := 2 + rand.Intn(8) // 2-9 的微小噪声
			if v > remaining {
				v = remaining
			}
			bins[i] = v
			remaining -= v
		}
	}

	// ── 余量归入 bins[0] (最大的 bin 微调不影响分布形状) ──
	if remaining > 0 {
		bins[0] += remaining
	} else if remaining < 0 {
		// 如果超出了, 从 bins[0] 扣除
		bins[0] += remaining // remaining is negative
		if bins[0] < 10000 {
			bins[0] = 10000
		}
	}

	// ── 计算 hash: SHA256(bins 的 LE 字节序列) 截取前 4 字节 → int32 ──
	raw := make([]byte, 256*4)
	for i, v := range bins {
		binary.LittleEndian.PutUint32(raw[i*4:], uint32(v))
	}
	digest := sha256.Sum256(raw)
	hash := int32(binary.LittleEndian.Uint32(digest[:4]))
	return hash, bins
}

// abs 整数绝对值
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// ──────────────────── 硬件档位: 保证参数关联合理 ────────────────────

// hardwareProfile 定义一组关联的硬件参数
type hardwareProfile struct {
	memoryOptions []int
	coreOptions   []int
	gpuTier       string // "low", "mid", "high"
	screenTier    string // "low", "mid", "high"
}

// 真实世界的硬件档位组合
var hwProfiles = []hardwareProfile{
	// 中端办公机 (最常见)
	{memoryOptions: []int{8, 16}, coreOptions: []int{4, 6, 8}, gpuTier: "mid", screenTier: "mid"},
	// 中高端工作站
	{memoryOptions: []int{16, 32}, coreOptions: []int{8, 12, 16}, gpuTier: "high", screenTier: "high"},
	// 轻薄本/入门机
	{memoryOptions: []int{8, 16}, coreOptions: []int{4, 8}, gpuTier: "low", screenTier: "mid"},
	// 高端游戏/开发机
	{memoryOptions: []int{32, 64}, coreOptions: []int{12, 16, 24}, gpuTier: "high", screenTier: "high"},
	// 普通家用机
	{memoryOptions: []int{8, 16}, coreOptions: []int{4, 6, 8}, gpuTier: "mid", screenTier: "mid"},
}

func genGPUByTier(tier string) (vendor, model string) {
	type gpuEntry struct {
		chipVendor string
		prefix     string
		model      string
	}

	lowGPUs := []gpuEntry{
		{"Intel", "Intel(R) ", "UHD Graphics 630"},
		{"Intel", "Intel(R) ", "UHD Graphics 730"},
		{"Intel", "Intel(R) ", "Iris(R) Xe Graphics"},
		{"Intel", "Intel(R) ", "UHD Graphics 620"},
		{"Intel", "Intel(R) ", "Iris(R) Plus Graphics"},
	}
	midGPUs := []gpuEntry{
		{"NVIDIA", "NVIDIA ", "GeForce GTX 1650"},
		{"NVIDIA", "NVIDIA ", "GeForce GTX 1660 Super"},
		{"NVIDIA", "NVIDIA ", "GeForce RTX 3060"},
		{"AMD", "AMD ", "Radeon RX 580"},
		{"AMD", "AMD ", "Radeon RX 5600 XT"},
		{"NVIDIA", "NVIDIA ", "GeForce RTX 2060"},
	}
	highGPUs := []gpuEntry{
		{"NVIDIA", "NVIDIA ", "GeForce RTX 3070"},
		{"NVIDIA", "NVIDIA ", "GeForce RTX 3080"},
		{"NVIDIA", "NVIDIA ", "GeForce RTX 4060"},
		{"NVIDIA", "NVIDIA ", "GeForce RTX 4070"},
		{"NVIDIA", "NVIDIA ", "GeForce RTX 4080"},
		{"AMD", "AMD ", "Radeon RX 6800 XT"},
		{"AMD", "AMD ", "Radeon RX 7800 XT"},
	}

	var pool []gpuEntry
	switch tier {
	case "low":
		pool = lowGPUs
	case "high":
		pool = highGPUs
	default:
		pool = midGPUs
	}

	g := pool[rand.Intn(len(pool))]
	vendor = fmt.Sprintf("Google Inc. (%s)", g.chipVendor)
	model = fmt.Sprintf("ANGLE (%s, %s%s Direct3D11 vs_5_0 ps_5_0, D3D11)", g.chipVendor, g.prefix, g.model)
	return
}

func genScreenByTier(tier string) ScreenInfo {
	type resolution struct{ w, h int }

	lowRes := []resolution{{1920, 1080}, {1600, 900}, {1536, 864}}
	midRes := []resolution{{1920, 1080}, {2560, 1440}, {1920, 1200}}
	highRes := []resolution{{2560, 1440}, {3840, 2160}, {3440, 1440}, {2560, 1600}}

	var pool []resolution
	switch tier {
	case "low":
		pool = lowRes
	case "high":
		pool = highRes
	default:
		pool = midRes
	}

	res := pool[rand.Intn(len(pool))]
	taskbar := 40 // Windows 标准任务栏高度

	return ScreenInfo{
		Width:       res.w,
		Height:      res.h,
		AvailWidth:  res.w,
		AvailHeight: res.h - taskbar,
		ColorDepth:  24,
	}
}

// ──────────────────── 核心: 随机身份生成 ────────────────────

// RandomIdentity 创建随机浏览器身份 (硬件参数关联合理)
func RandomIdentity() *BrowserIdentity {
	// Chrome 版本
	cv := genChromeVersion()

	// 选择硬件档位，保证参数关联
	profile := hwProfiles[rand.Intn(len(hwProfiles))]
	deviceMemory := profile.memoryOptions[rand.Intn(len(profile.memoryOptions))]
	hardwareConcurrency := profile.coreOptions[rand.Intn(len(profile.coreOptions))]

	// GPU 和屏幕与档位关联
	gpuVendor, gpuModel := genGPUByTier(profile.gpuTier)
	screen := genScreenByTier(profile.screenTier)

	platform := "Win32"

	// Math 精度 (真实值微小变化)
	mathTan, mathSin, mathCos := genMath()

	// Canvas 数据 (算法模拟)
	canvasHash, histogram := generateCanvasData()

	// WebGL 扩展
	exts := make([]string, len(webglExtCore))
	copy(exts, webglExtCore)
	nOpt := 1 + rand.Intn(4) // 至少 1 个可选扩展
	if nOpt <= len(webglExtOptional) {
		perm := rand.Perm(len(webglExtOptional))
		for i := 0; i < nOpt; i++ {
			exts = append(exts, webglExtOptional[perm[i]])
		}
	}
	sort.Strings(exts)

	// 插件 (Chrome 内置 PDF 插件, 随机排列)
	plugins := make([]map[string]string, len(pluginsPool))
	copy(plugins, pluginsPool)
	rand.Shuffle(len(plugins), func(i, j int) { plugins[i], plugins[j] = plugins[j], plugins[i] })

	// 美区时区: -5(EST), -6(CST), -7(MST), -8(PST)
	usTimezones := []int{-5, -6, -7, -8}
	timeZone := usTimezones[rand.Intn(len(usTimezones))]

	ua := fmt.Sprintf("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s Safari/537.36", cv.Version)

	return &BrowserIdentity{
		ChromeVer:           cv.Version,
		UA:                  ua,
		SecUA:               cv.SecUA,
		GPUVendor:           gpuVendor,
		GPUModel:            gpuModel,
		WebGLExts:           exts,
		CanvasHash:          canvasHash,
		HistogramBase:       histogram,
		MathTan:             mathTan,
		MathSin:             mathSin,
		MathCos:             mathCos,
		Plugins:             plugins,
		Screen:              screen,
		DeviceMemory:        deviceMemory,
		HardwareConcurrency: hardwareConcurrency,
		Platform:            platform,
		TimeZone:            timeZone,
		LsubidPrefixSignin:  lsubidPrefixes[rand.Intn(len(lsubidPrefixes))],
		LsubidPrefixProfile: lsubidPrefixes[rand.Intn(len(lsubidPrefixes))],
		WebpackHash:         fmt.Sprintf("%x", rand.Int63())[:10],
	}
}
