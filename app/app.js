const {
  computed,
  createApp,
  onBeforeUnmount,
  onMounted,
  reactive,
  ref,
  watch,
} = Vue;
const TABLE_DEFINITIONS = window.OhWembyTableDefinitions;

const CHINESE_TRANSLATIONS = Object.freeze({
  Overview: "概览",
  Tasks: "任务",
  Devices: "设备",
  Projects: "项目",
  "Test Cases": "测试用例",
  "Test Suites": "测试套件",
  "New Test Run": "新建测试运行",
  Activity: "活动",
  Settings: "设置",
  "Primary navigation": "主导航",
  "New test case": "新建测试用例",
  "New test suite": "新建测试套件",
  "New test run": "新建测试运行",
  "Start test run": "开始测试运行",
  "Add demo task": "添加演示任务",
  "Vue is ready": "Vue 已准备就绪",
  "A clear starting point for the next big thing.": "为下一项重要工作提供清晰起点。",
  "This first screen establishes the visual direction, confirms the local runtime, and gives us a working surface to build on.": "此初始页面确立视觉方向、确认本地运行环境，并提供可继续构建的工作界面。",
  "Create a task": "创建任务",
  "View tasks": "查看任务",
  "View activity": "查看活动",
  "Build status": "构建状态",
  Healthy: "正常",
  "Project statistics": "项目统计",
  "Project progress": "项目进度",
  "Completed tasks": "已完成任务",
  "Current focus": "当前重点",
  Foundation: "基础建设",
  "Ready for product requirements": "已准备好接收产品需求",
  "Getting started": "开始使用",
  "Launch checklist": "启动检查清单",
  Owner: "负责人",
  Done: "已完成",
  Next: "下一步",
  "Recent updates": "最近更新",
  "Demo page created": "演示页面已创建",
  "Vue and Element Plus are connected.": "Vue 与 Element Plus 已连接。",
  "Project rules confirmed": "项目规则已确认",
  "Stable dependencies and one fixed port.": "使用稳定依赖项和一个固定端口。",
  "Workspace initialized": "工作区已初始化",
  "The foundation is ready to extend.": "基础功能已准备好扩展。",
  "Release 2.4 regression": "2.4 版本回归测试",
  "Release quality overview": "版本质量概览",
  "See what’s ready. Fix what’s not.": "看清就绪项，解决待处理问题。",
  "Track tests, review issues, and start new runs.": "跟踪测试、审查问题并启动新的测试运行。",
  "Release 2.4 · QA staging": "2.4 版本 · QA 预发布环境",
  "Release confidence": "版本置信度",
  "Based on executed checks": "基于已执行的检查",
  "Runs to review": "待审查运行",
  "Failed or blocked checks": "包含失败或受阻检查",
  "Running now": "正在运行",
  "Ready to run": "待运行",
  "Awaiting execution": "等待执行",
  "Recent validation": "最近验证",
  "Latest test runs across the release.": "该版本最新的测试运行。",
  "View all runs": "查看全部运行",
  "Attention needed": "需要关注",
  "Review these runs before release approval.": "请在批准发布前审查这些运行。",
  "No runs need attention": "没有需要关注的运行",
  "All recorded checks are clear.": "所有已记录的检查均无异常。",
  "Open run": "打开运行",
  "checks executed": "项检查已执行",
  "Quality at a glance": "质量概览",
  "Track coverage, review failures, and keep every release decision tied to a clear test result.": "跟踪覆盖范围、审查失败项，并确保每项发布决策都有明确的测试结果依据。",
  "Pass rate": "通过率",
  Passing: "通过",
  "Needs attention": "需要关注",
  "failed, blocked or pending": "失败、受阻或待执行",
  "Test library": "测试库",
  "Standard test cases": "标准测试用例",
  "Customized test cases": "自定义测试用例",
  Standard: "标准",
  Customized: "自定义",
  "Contact an administrator to access customized test cases.": "如需访问自定义测试用例，请联系管理员。",
  "Test cases formally maintained by the testing team.": "由测试团队正式维护的测试用例。",
  "Supplementary test cases contributed by developers and other team members.": "由开发人员及其他团队成员补充的测试用例。",
  "Search by ID, title, or owner": "按 ID、标题或负责人搜索",
  "Add to Test Suite": "添加到测试套件",
  "Create Test Run": "创建测试运行",
  Clear: "清除",
  "No test cases match these filters": "没有符合筛选条件的测试用例",
  Edit: "编辑",
  "Contact an administrator to edit test cases.": "如需编辑测试用例，请联系管理员。",
  "Reusable test case groups": "可复用的测试用例组",
  "Build once, run together.": "一次构建，统一运行。",
  "Organize related test cases into suites, then select one or more suites when starting a focused test run.": "将相关测试用例整理为套件，并在启动专项测试运行时选择一个或多个套件。",
  "Test suite summary": "测试套件摘要",
  "Test suites": "测试套件",
  "Grouped cases": "已分组用例",
  "Suite library": "套件库",
  "All test suites": "全部测试套件",
  "Search by suite, ID, owner, or description": "按套件、ID、负责人或描述搜索",
  "Create suite": "创建套件",
  "No test suites match this search": "没有符合搜索条件的测试套件",
  Run: "运行",
  "Test run is active": "测试运行进行中",
  "Open test cases": "打开测试用例",
  "Configure another run": "配置另一次运行",
  "Quality assurance workflow": "质量保证工作流",
  "Start a focused test run.": "启动专项测试运行。",
  "Name the run, choose a target device, and select the test cases for the tester.": "命名测试运行、选择目标设备，并为测试人员选择测试用例。",
  "Test run name": "测试运行名称",
  "Example: Release 2.4 regression": "示例：2.4 版本回归测试",
  Device: "设备",
  "Connected devices": "已连接设备",
  "Devices detected through HDC": "通过 HDC 检测到的设备",
  "This page searches the computer for connected devices by running HDC.": "此页面通过运行 HDC 搜索连接到电脑的设备。",
  "Test devices": "测试设备",
  "See every device ready for testing.": "查看所有已准备好进行测试的设备。",
  "Find devices connected through HDC and confirm each target before starting a run.": "查找通过 HDC 连接的设备，并在开始运行前确认每个测试目标。",
  "Refresh devices": "刷新设备",
  "Searching for devices...": "正在搜索设备……",
  "No connected devices found": "未找到已连接的设备",
  "Connect a device, confirm that HDC can access it, and refresh the list.": "请连接设备，确认 HDC 可以访问该设备，然后刷新列表。",
  "Device identifier": "设备标识符",
  Model: "型号",
  "OS version": "系统版本",
  Type: "类型",
  Unknown: "未知",
  Connected: "已连接",
  "Device search failed": "设备搜索失败",
  "Try again after resolving the HDC issue shown below.": "解决下方显示的 HDC 问题后，请重试。",
  "Search by model, browser, or device ID": "按型号、浏览器或设备 ID 搜索",
  "No matching devices": "没有匹配的设备",
  "Test selection": "测试选择",
  "Test cases": "测试用例",
  "Search and select test cases": "搜索并选择测试用例",
  "Search and select test suites": "搜索并选择测试套件",
  "No matching test cases": "没有匹配的测试用例",
  "Choose one or more test cases for this run.": "为此次运行选择一个或多个测试用例。",
  "No matching test suites": "没有匹配的测试套件",
  Notes: "备注",
  "Add optional instructions or context": "添加可选说明或背景信息",
  "The run begins in a ready state; no tests execute automatically.": "测试运行将进入就绪状态，不会自动执行任何测试。",
  "Run preview": "运行预览",
  "Untitled test run": "未命名的测试运行",
  "Target device": "目标设备",
  "No device selected": "未选择设备",
  "Ready to start": "可以开始",
  "Setup incomplete": "设置未完成",
  "The test run can now be started.": "现在可以开始测试运行。",
  "Complete the three required fields.": "请完成三个必填字段。",
  "This area is ready for its next workflow.": "此区域已准备好承载下一个工作流。",
  "Edit test case": "编辑测试用例",
  "Create test case": "创建测试用例",
  "Test case title": "测试用例标题",
  "Describe the behavior to verify": "描述需要验证的行为",
  "Name or team": "姓名或团队",
  Cancel: "取消",
  "Save changes": "保存更改",
  "Edit test suite": "编辑测试套件",
  "Create test suite": "创建测试套件",
  "Suite name": "套件名称",
  "Example: Release 2.5 critical path": "示例：2.5 版本关键路径",
  Description: "描述",
  "Explain when this suite should be used": "说明此套件的适用场景",
  "Select at least one test case.": "请至少选择一个测试用例。",
  "Enter a test case title.": "请输入测试用例标题。",
  "Test case updated.": "测试用例已更新。",
  "Test case created.": "测试用例已创建。",
  "Enter a test suite name.": "请输入测试套件名称。",
  "Test suite updated.": "测试套件已更新。",
  "Test suite created.": "测试套件已创建。",
  "Enter a test run name.": "请输入测试运行名称。",
  "Select a device.": "请选择设备。",
  "Demo task added.": "演示任务已添加。",
  "New task": "新建任务",
  "Application settings": "应用设置",
  "Edit the runtime parameters stored in the local config file.": "编辑保存在本地配置文件中的运行参数。",
  "Config file": "配置文件",
  "Project name": "项目名称",
  "Release name": "版本名称",
  "Default environment": "默认环境",
  "Default owner": "默认负责人",
  "Test case library path": "测试用例库路径",
  "Python executable path": "Python 可执行文件路径",
  "Include all .py files in this directory and its subdirectories.": "包含此目录及其所有子目录中的 .py 文件。",
  "Must point to a python.exe file.": "必须指向 python.exe 文件。",
  "Auto-load devices": "自动加载设备",
  "Refresh interval": "刷新间隔",
  "Table page size": "表格每页数量",
  "seconds": "秒",
  "Saving...": "正在保存……",
  "Saved": "已保存",
  "Settings unavailable": "设置不可用",
  "Reload settings": "重新加载设置",
  "Reset defaults": "恢复默认值",
  "Settings restored to defaults.": "设置已恢复为默认值。",
  "Settings saved.": "设置已保存。",
  "Inspection mode": "检查模式",
  "Execute only": "仅执行",
  "Static inspection": "静态检测",
  "Motion inspection": "动效检测",
  "Test case library unavailable": "测试用例库不可用",
  "Reload test cases": "重新加载测试用例",
  "Starting...": "正在启动……",
  "Test run failed": "测试运行启动失败",
  "Task command center": "任务指挥中心",
  "Plan, assign, and deliver release work from one focused workspace.": "在一个专注的工作区中规划、分配并交付发布工作。",
  "Total tasks": "任务总数",
  "In progress": "进行中",
  "Overdue": "已逾期",
  "Completion rate": "完成率",
  "Task board": "任务看板",
  "Search by task, ID, owner, or project": "按任务、ID、负责人或项目搜索",
  "All statuses": "全部状态",
  "All priorities": "全部优先级",
  "No tasks match these filters": "没有符合筛选条件的任务",
  "Task": "任务",
  "Status": "状态",
  "Priority": "优先级",
  "Assignee": "负责人",
  "Due date": "截止日期",
  "Progress": "进度",
  "Task details": "任务详情",
  "Select a task to inspect its scope and delivery details.": "选择任务以查看其范围和交付详情。",
  "Project": "项目",
  "Updated": "更新时间",
  "Deliverables": "交付内容",
  "Activity report": "活动报告",
  "Delivery health": "交付健康度",
  "On track": "进展正常",
  "Tasks by status": "按状态统计任务",
  "Team workload": "团队工作量",
  "Upcoming milestone": "即将到来的里程碑",
  "Release candidate review": "发布候选版本评审",
  "Create task": "创建任务",
  "Task title": "任务标题",
  "What needs to be delivered?": "需要交付什么？",
  "Choose status": "选择状态",
  "Choose priority": "选择优先级",
  "Assignee name": "负责人姓名",
  "Due": "截止",
  "Save task": "保存任务",
  "Enter a task title.": "请输入任务标题。",
  "Task created.": "任务已创建。",
  "Open": "待开始",
  "In Progress": "进行中",
  "In Review": "评审中",
  "Completed": "已完成",
  "High": "高",
  "Medium": "中",
  "Low": "低",
  "Today": "今天",
  "Tomorrow": "明天",
  "Release 2.4": "2.4 版本",
  "Mobile checkout": "移动端结账",
  "Reporting refresh": "报告刷新",
  "Authentication hardening": "身份验证加固",
  "Stabilize checkout regression suite": "稳定结账回归测试套件",
  "Verify the critical checkout path across supported devices before the release candidate review.": "在发布候选版本评审前，验证所有受支持设备上的关键结账流程。",
  "Resolve failed payment retry checks": "解决付款重试检查失败问题",
  "Document release acceptance criteria": "记录发布验收标准",
  "Validate SSO session expiry behavior": "验证单点登录会话过期行为",
  "Review dashboard export coverage": "审查仪表板导出覆盖范围",
  "Prepare mobile device test matrix": "准备移动设备测试矩阵",
  "Publish release readiness report": "发布版本就绪报告",
  "Triage blocked authentication cases": "梳理受阻的身份验证用例",
  "Checkout flow verified": "结账流程已验证",
  "Failure evidence attached": "已附上失败证据",
  "Release notes updated": "发布说明已更新",
  "Device coverage confirmed": "设备覆盖范围已确认",
  "Acceptance criteria approved": "验收标准已批准",
  "Final report shared": "最终报告已共享",
  "Test runs": "测试运行",
  "Test run history": "测试运行历史",
  "Recent test runs": "最近的测试运行",
  "Monitor active runs, review outcomes, and keep release validation moving.": "监控正在执行的测试、审查结果，并持续推进版本验证。",
  "Active runs": "进行中的运行",
  "Completed runs": "已完成的运行",
  "Overall pass rate": "总体通过率",
  "Search by run, ID, device, suite, or owner": "按运行、ID、设备、套件或负责人搜索",
  "All run statuses": "全部运行状态",
  "No test runs match these filters": "没有符合筛选条件的测试运行",
  "Run": "运行",
  "Device": "设备",
  "Results": "结果",
  "Run details": "运行详情",
  "Execution summary": "执行摘要",
  "Test suite": "测试套件",
  "Environment": "环境",
  "Duration": "持续时间",
  "Executed": "已执行",
  "Total": "总计",
  "Running": "运行中",
  "Ready": "就绪",
  "View test cases": "查看测试用例",
  "Release 2.4 full regression": "2.4 版本完整回归测试",
  "Checkout critical-path smoke test": "结账关键路径冒烟测试",
  "Authentication recovery validation": "身份验证恢复流程验证",
  "Reporting and export acceptance": "报告与导出验收测试",
  "Payment retry regression": "付款重试回归测试",
  "Session security verification": "会话安全验证",
  "Full release-candidate coverage across authentication, checkout, reporting, and security workflows.": "覆盖身份验证、结账、报告和安全流程的完整发布候选版本测试。",
  "Fast confidence check for the purchase path before the release candidate is promoted.": "在发布候选版本升级前，对购买路径执行快速置信度检查。",
  "Recovery, lockout, and password-reset coverage is paused while the test account is restored.": "在恢复测试账户期间，恢复、锁定和密码重置相关测试已暂停。",
  "Acceptance coverage for dashboard filters, monthly reports, and exported CSV files.": "针对仪表板筛选、月度报告和 CSV 导出文件的验收覆盖。",
  "Regression coverage for failed payments, invoice preservation, and retry behavior.": "针对付款失败、发票保留和重试行为的回归测试。",
  "Session timeout, restricted-route, and audit-log checks for the release candidate.": "针对发布候选版本的会话超时、受限路由和审计日志检查。",
  "QA staging · build 2.4.0-rc.3": "QA 预发布环境 · 构建 2.4.0-rc.3",
  "QA staging · build 2.4.0-rc.2": "QA 预发布环境 · 构建 2.4.0-rc.2",
  "Completed without blocking issues.": "已完成，无阻塞问题。",
  "One blocked case requires a restored test account before execution can continue.": "一个受阻用例需要恢复测试账户后才能继续执行。",
  "No failures recorded": "未记录失败",
  "Review the recorded failures before closing this run.": "关闭此次运行前，请审查已记录的失败项。",
  "Test run details": "测试运行详情",
  "Back to test runs": "返回测试运行",
  "View": "查看",
  Passed: "通过",
  Failed: "失败",
  Blocked: "受阻",
  "Not run": "未运行",
  Unassigned: "未分配",
  "Just now": "刚刚",
  Yesterday: "昨天",
  ID: "ID",
  Title: "标题",
  Updated: "更新时间",
  Actions: "操作",
  Name: "名称",
  "Confirm product direction": "确认产品方向",
  "Build the first Vue screen": "构建首个 Vue 页面",
  "Connect real project data": "连接真实项目数据",
  "Review the first workflow": "审查首个工作流",
  "User can sign in with valid credentials": "用户可以使用有效凭据登录",
  "Locked account displays recovery guidance": "锁定的账户会显示恢复指引",
  "Checkout preserves items after refresh": "刷新后结账页面仍保留商品",
  "Coupon rejects an expired promotion code": "优惠券会拒绝已过期的促销代码",
  "Admin can export the monthly usage report": "管理员可以导出月度使用报告",
  "Session expires after inactivity threshold": "会话在超过非活动时限后过期",
  "User can sign in with single sign-on": "用户可以通过单点登录进行登录",
  "Password reset link expires after use": "密码重置链接使用后失效",
  "Order total includes the correct regional tax": "订单总额包含正确的地区税费",
  "Guest checkout accepts a valid shipping address": "访客结账接受有效的配送地址",
  "Dashboard date filter updates every chart": "仪表板日期筛选器会更新所有图表",
  "CSV export preserves visible column order": "CSV 导出会保留可见列的顺序",
  "Restricted route rejects an expired session": "受限路由会拒绝已过期的会话",
  "Audit log records permission changes": "审计日志会记录权限变更",
  "User can update their preferred display name": "用户可以更新首选显示名称",
  "Avatar upload rejects unsupported file types": "头像上传会拒绝不支持的文件类型",
  "Email preference is saved immediately": "电子邮件偏好设置会立即保存",
  "Unread count clears after opening the inbox": "打开收件箱后未读计数会清零",
  "Invoice displays the active subscription plan": "发票会显示当前订阅方案",
  "Payment retry preserves the original invoice": "重试付款会保留原始发票",
  "Webhook delivery retries after a timeout": "Webhook 投递会在超时后重试",
  "Disconnected provider prompts reconnection": "服务提供商断开后会提示重新连接",
  Authentication: "身份验证",
  Checkout: "结账",
  Reporting: "报告",
  Security: "安全",
  Profile: "个人资料",
  Notifications: "通知",
  Billing: "计费",
  Integrations: "集成",
  Smoke: "冒烟测试",
  Regression: "回归测试",
  "Critical Path": "关键路径",
  Acceptance: "验收测试",
  "Release Readiness": "发布就绪",
  "Cross-browser": "跨浏览器",
  "Production Validation": "生产验证",
  "Edge Cases": "边界情况",
});

const GENERATED_TEST_SCENARIOS = [
  ["Authentication", "User can sign in with single sign-on"],
  ["Authentication", "Password reset link expires after use"],
  ["Checkout", "Order total includes the correct regional tax"],
  ["Checkout", "Guest checkout accepts a valid shipping address"],
  ["Reporting", "Dashboard date filter updates every chart"],
  ["Reporting", "CSV export preserves visible column order"],
  ["Security", "Restricted route rejects an expired session"],
  ["Security", "Audit log records permission changes"],
  ["Profile", "User can update their preferred display name"],
  ["Profile", "Avatar upload rejects unsupported file types"],
  ["Notifications", "Email preference is saved immediately"],
  ["Notifications", "Unread count clears after opening the inbox"],
  ["Billing", "Invoice displays the active subscription plan"],
  ["Billing", "Payment retry preserves the original invoice"],
  ["Integrations", "Webhook delivery retries after a timeout"],
  ["Integrations", "Disconnected provider prompts reconnection"],
];
const GENERATED_OWNERS = [
  "Maya Chen",
  "Jon Bell",
  "Priya Shah",
  "Alex Kim",
  "Nora Diaz",
  "Sam Wilson",
];
const GENERATED_STATUSES = [
  "Passed",
  "Passed",
  "Passed",
  "Failed",
  "Blocked",
  "Not run",
];
const GENERATED_SUITE_AREAS = [
  "Authentication",
  "Checkout",
  "Reporting",
  "Security",
  "Profile",
  "Notifications",
  "Billing",
  "Integrations",
];
const GENERATED_SUITE_SCOPES = [
  "Smoke",
  "Regression",
  "Critical Path",
  "Acceptance",
  "Release Readiness",
  "Cross-browser",
  "Production Validation",
  "Edge Cases",
];
function updatedSortValue(updated) {
  const value = updated.toLowerCase();

  if (value === "just now") {
    return 0;
  }

  if (value.includes("min")) {
    return Number.parseInt(value, 10) || 0;
  }

  if (value.includes("hr")) {
    return (Number.parseInt(value, 10) || 0) * 60;
  }

  if (value === "yesterday") {
    return 24 * 60;
  }

  if (value.includes("day")) {
    return (Number.parseInt(value, 10) || 1) * 24 * 60;
  }

  const timestamp = Date.parse(updated);
  if (!Number.isNaN(timestamp)) {
    return -timestamp;
  }

  return Number.MAX_SAFE_INTEGER;
}

function testCaseIdSortValue(id) {
  return Number.parseInt(id.replace(/\D/g, ""), 10) || 0;
}

function buildGeneratedTestCases(count) {
  return Array.from({ length: count }, (_, index) => {
    const [, scenario] =
      GENERATED_TEST_SCENARIOS[index % GENERATED_TEST_SCENARIOS.length];
    const variation = Math.floor(index / GENERATED_TEST_SCENARIOS.length) + 1;

    return {
      id: `TC-${1043 + index}`,
      title: variation === 1 ? scenario : `${scenario} - variation ${variation}`,
      category: index % 9 === 8 ? "Customized" : "Standard",
      status: GENERATED_STATUSES[index % GENERATED_STATUSES.length],
      owner: GENERATED_OWNERS[index % GENERATED_OWNERS.length],
      updated: `${(index % 24) + 1} hrs ago`,
    };
  });
}

function pickCaseIds(testCases, startIndex, count, step = 1) {
  return Array.from(
    { length: Math.min(count, testCases.length) },
    (_, offset) => testCases[(startIndex + offset * step) % testCases.length].id,
  );
}

function buildGeneratedTestSuites(count, testCases) {
  if (!testCases.length) {
    return [];
  }

  return Array.from({ length: count }, (_, index) => {
    const area = GENERATED_SUITE_AREAS[index % GENERATED_SUITE_AREAS.length];
    const scope =
      GENERATED_SUITE_SCOPES[
        Math.floor(index / GENERATED_SUITE_AREAS.length) %
          GENERATED_SUITE_SCOPES.length
      ];
    const cycle =
      Math.floor(
        index /
          (GENERATED_SUITE_AREAS.length * GENERATED_SUITE_SCOPES.length),
      ) + 1;
    const caseCount = 4 + ((index * 7) % 17);
    const startIndex = (index * 11) % testCases.length;
    const caseIds = Array.from(
      { length: Math.min(caseCount, testCases.length) },
      (_, offset) => testCases[(startIndex + offset * 3) % testCases.length].id,
    );

    return {
      id: `TS-${206 + index}`,
      name: `${area} ${scope}${cycle > 1 ? ` ${cycle}` : ""}`,
      description: `${scope} coverage for ${area.toLowerCase()} workflows and release validation.`,
      owner: GENERATED_OWNERS[(index * 5) % GENERATED_OWNERS.length],
      caseIds: [...new Set(caseIds)],
      updated:
        index % 9 === 0
          ? "Yesterday"
          : `${(index % 23) + 1} hrs ago`,
    };
  });
}

const HISTORICAL_RUNS = [
  ["Release candidate regression", "Release 2.4 Critical Path"],
  ["Checkout device coverage", "Checkout Regression"],
  ["Authentication smoke validation", "Authentication Smoke"],
  ["Reporting export verification", "Reporting and Export"],
  ["Security access review", "Security and Access Control"],
  ["Production readiness check", "Production Validation"],
];
const HISTORICAL_RUN_DEVICES = [
  "Pura 70 Pro",
  "Mate 60 Pro",
  "Nova 12",
  "Mate X5",
  "Pura 70",
  "Mate 60",
];

const DEFAULT_APP_SETTINGS = Object.freeze({
  projectName: "Oh Wemby",
  releaseName: "Release 2.4",
  defaultEnvironment: "QA staging",
  defaultOwner: "kouyanan 30030842",
  testCaseLibraryPath: "",
  pythonExecutablePath: "",
  autoLoadDevices: true,
  deviceRefreshSeconds: 30,
  tablePageSize: 20,
});

function buildHistoricalTestRuns(count) {
  return Array.from({ length: count }, (_, index) => {
    const [title, suite] = HISTORICAL_RUNS[index % HISTORICAL_RUNS.length];
    const totalCases = 12 + ((index * 7) % 29);
    const blockedRun = index % 9 === 8;
    const failed = blockedRun ? 0 : index % 4;
    const blocked = blockedRun ? 1 : 0;
    const executedCases = blockedRun
      ? Math.max(4, Math.floor(totalCases * 0.4))
      : totalCases;
    const passed = Math.max(0, executedCases - failed - blocked);

    return {
      id: `TR-${String(18 - index).padStart(3, "0")}`,
      title: `${title} · cycle ${Math.floor(index / HISTORICAL_RUNS.length) + 1}`,
      description: `${title} completed as part of the release validation schedule.`,
      suite,
      device: HISTORICAL_RUN_DEVICES[index % HISTORICAL_RUN_DEVICES.length],
      environment: "QA staging · build 2.4.0-rc.2",
      owner: GENERATED_OWNERS[index % GENERATED_OWNERS.length],
      status: blockedRun ? "Blocked" : "Completed",
      totalCases,
      executedCases,
      passed,
      failed,
      blocked,
      progress: blockedRun
        ? Math.round((executedCases / totalCases) * 100)
        : 100,
      duration: `${22 + (index % 31)} min`,
      updated: `${2 + Math.floor(index / 3)} days ago`,
      done: !blockedRun,
    };
  });
}

const App = {
  setup() {
    const language = ref(
      window.localStorage.getItem("oh-wemby-language") === "zh-CN"
        ? "zh-CN"
        : "en",
    );
    const isChinese = computed(() => language.value === "zh-CN");
    const t = (text) =>
      isChinese.value ? CHINESE_TRANSLATIONS[text] || text : text;
    const displayValue = (value) => {
      if (!isChinese.value || typeof value !== "string") {
        return value;
      }
      if (CHINESE_TRANSLATIONS[value]) {
        return CHINESE_TRANSLATIONS[value];
      }
      const relativeTime = value.match(/^(\d+) (min|hr|hrs|day|days) ago$/);
      if (relativeTime) {
        const units = { min: "分钟", hr: "小时", hrs: "小时", day: "天", days: "天" };
        return `${relativeTime[1]} ${units[relativeTime[2]]}前`;
      }
      const variation = value.match(/^(.*) - variation (\d+)$/);
      if (variation) {
        return `${displayValue(variation[1])} - 变体 ${variation[2]}`;
      }
      let translated = value;
      for (const [english, chinese] of Object.entries(CHINESE_TRANSLATIONS)) {
        translated = translated.split(english).join(chinese);
      }
      return translated;
    };
    const initialSearchParams = new URLSearchParams(window.location.search);
    const requestedView = initialSearchParams.get("view");
    const requestedRunId = initialSearchParams.get("run");
    const requestedViews = {
      devices: "Devices",
      "test-cases": "Test Cases",
      "test-suites": "Test Suites",
      "new-test-run": "New Test Run",
      tasks: "Tasks",
      "test-run": "Test Run Details",
      settings: "Settings",
    };
    const viewRoutes = {
      Overview: "",
      Devices: "?view=devices",
      "Test Cases": "?view=test-cases",
      "Test Suites": "?view=test-suites",
      "New Test Run": "?view=new-test-run",
      Tasks: "?view=tasks",
      Settings: "?view=settings",
    };
    const activeView = ref(requestedViews[requestedView] || "Overview");
    const appSettings = reactive({ ...DEFAULT_APP_SETTINGS });
    const settingsLoading = ref(false);
    const settingsSaving = ref(false);
    const settingsError = ref("");
    const settingsSavedAt = ref("");
    let settingsLoaded = false;
    let applyingSettings = false;
    let settingsSaveTimer = 0;
    const devices = ref([]);
    const devicesLoading = ref(false);
    const devicesError = ref("");
    const devicesLoaded = ref(false);
    const searchQuery = ref("");
    const testCaseCategory = ref("Standard");
    const testCaseSort = reactive({
      prop: "updated",
      order: "ascending",
    });
    const currentPage = ref(1);
    const pageSize = ref(20);
    const testCaseTableRef = ref(null);
    const selectedTestCases = ref([]);
    const createDialogVisible = ref(false);
    const createSuiteDialogVisible = ref(false);
    const editingTestCaseId = ref("");
    const editingTestSuiteId = ref("");
    const suiteSearchQuery = ref("");
    const suiteCurrentPage = ref(1);
    const suitePageSize = ref(20);
    const testRunStarted = ref(false);
    const testRunStarting = ref(false);
    const testRunError = ref("");
    const testRunExecution = ref(null);
    const testCasesLoading = ref(false);
    const testCasesError = ref("");
    const testRunSearchQuery = ref("");
    const testRunStatusFilter = ref("");
    const testRunCurrentPage = ref(1);
    const testRunPageSize = ref(20);
    const selectedTestRunId = ref(requestedRunId || "TR-024");
    const testRuns = ref([
      {
        id: "TR-024",
        title: "Release 2.4 full regression",
        description:
          "Full release-candidate coverage across authentication, checkout, reporting, and security workflows.",
        suite: "Release 2.4 Critical Path",
        device: "Pura 70 Pro",
        environment: "QA staging · build 2.4.0-rc.3",
        owner: "Maya Chen",
        status: "Running",
        totalCases: 48,
        executedCases: 31,
        passed: 28,
        failed: 2,
        blocked: 1,
        progress: 65,
        duration: "42 min",
        updated: "12 min ago",
        done: false,
      },
      {
        id: "TR-023",
        title: "Checkout critical-path smoke test",
        description:
          "Fast confidence check for the purchase path before the release candidate is promoted.",
        suite: "Checkout Regression",
        device: "Mate 60 Pro",
        environment: "QA staging · build 2.4.0-rc.3",
        owner: "Priya Shah",
        status: "Completed",
        totalCases: 14,
        executedCases: 14,
        passed: 14,
        failed: 0,
        blocked: 0,
        progress: 100,
        duration: "18 min",
        updated: "38 min ago",
        done: true,
      },
      {
        id: "TR-022",
        title: "Authentication recovery validation",
        description:
          "Recovery, lockout, and password-reset coverage is paused while the test account is restored.",
        suite: "Authentication Smoke",
        device: "Nova 12",
        environment: "QA staging · build 2.4.0-rc.3",
        owner: "Jon Bell",
        status: "Blocked",
        totalCases: 24,
        executedCases: 8,
        passed: 7,
        failed: 0,
        blocked: 1,
        progress: 33,
        duration: "26 min",
        updated: "1 hr ago",
        done: false,
      },
      {
        id: "TR-021",
        title: "Reporting and export acceptance",
        description:
          "Acceptance coverage for dashboard filters, monthly reports, and exported CSV files.",
        suite: "Reporting and Export",
        device: "Pura 70",
        environment: "QA staging · build 2.4.0-rc.3",
        owner: "Alex Kim",
        status: "Ready",
        totalCases: 18,
        executedCases: 0,
        passed: 0,
        failed: 0,
        blocked: 0,
        progress: 0,
        duration: "—",
        updated: "2 hrs ago",
        done: false,
      },
      {
        id: "TR-020",
        title: "Payment retry regression",
        description:
          "Regression coverage for failed payments, invoice preservation, and retry behavior.",
        suite: "Checkout Regression",
        device: "Mate X5",
        environment: "QA staging · build 2.4.0-rc.2",
        owner: "Nora Diaz",
        status: "Completed",
        totalCases: 16,
        executedCases: 16,
        passed: 15,
        failed: 1,
        blocked: 0,
        progress: 100,
        duration: "31 min",
        updated: "Yesterday",
        done: true,
      },
      {
        id: "TR-019",
        title: "Session security verification",
        description:
          "Session timeout, restricted-route, and audit-log checks for the release candidate.",
        suite: "Security and Access Control",
        device: "Mate 60",
        environment: "QA staging · build 2.4.0-rc.2",
        owner: "Sam Wilson",
        status: "Completed",
        totalCases: 20,
        executedCases: 20,
        passed: 20,
        failed: 0,
        blocked: 0,
        progress: 100,
        duration: "29 min",
        updated: "Yesterday",
        done: true,
      },
      ...buildHistoricalTestRuns(18),
    ]);
    const tasks = testRuns;
    const testCases = ref([]);
    const testSuites = ref([
      {
        id: "TS-201",
        name: "Release 2.4 Critical Path",
        description:
          "Business-critical authentication, checkout, reporting, and security checks.",
        owner: "Maya Chen",
        caseIds: pickCaseIds(testCases.value, 0, 12, 5),
        updated: "18 min ago",
      },
      {
        id: "TS-202",
        name: "Authentication Smoke",
        description:
          "Fast confidence checks for sign-in, recovery, and session access.",
        owner: "Jon Bell",
        caseIds: pickCaseIds(testCases.value, 1, 10, 8),
        updated: "2 hrs ago",
      },
      {
        id: "TS-203",
        name: "Checkout Regression",
        description:
          "Payment, tax, cart persistence, promotions, and guest checkout coverage.",
        owner: "Priya Shah",
        caseIds: pickCaseIds(testCases.value, 2, 14, 8),
        updated: "Yesterday",
      },
      {
        id: "TS-204",
        name: "Reporting and Export",
        description:
          "Dashboard filters, monthly reporting, and exported file validation.",
        owner: "Alex Kim",
        caseIds: pickCaseIds(testCases.value, 4, 12, 8),
        updated: "Yesterday",
      },
      {
        id: "TS-205",
        name: "Security and Access Control",
        description:
          "Session expiry, restricted routes, permissions, and audit coverage.",
        owner: "Jon Bell",
        caseIds: pickCaseIds(testCases.value, 5, 12, 8),
        updated: "2 days ago",
      },
      ...buildGeneratedTestSuites(200, testCases.value),
    ]);
    const newTestCase = reactive({
      title: "",
      owner: "",
    });
    const newTestSuite = reactive({
      name: "",
      description: "",
      owner: "",
      caseIds: [],
    });
    const newTestRun = reactive({
      name: "",
      device: "",
      selectionType: "Test cases",
      selections: [],
      inspectionMode: 0,
      notes: "",
    });

    const filteredTestRuns = computed(() => {
      const query = testRunSearchQuery.value.trim().toLowerCase();

      return testRuns.value.filter((run) => {
        const matchesSearch =
          !query ||
          [run.id, run.title, run.owner, run.device, run.suite].some((value) =>
            value.toLowerCase().includes(query),
          );
        const matchesStatus =
          !testRunStatusFilter.value || run.status === testRunStatusFilter.value;
        return matchesSearch && matchesStatus;
      });
    });
    const paginatedTestRuns = computed(() => {
      const start = (testRunCurrentPage.value - 1) * testRunPageSize.value;
      return filteredTestRuns.value.slice(start, start + testRunPageSize.value);
    });
    const selectedTestRun = computed(
      () =>
        testRuns.value.find((run) => run.id === selectedTestRunId.value) ||
        null,
    );
    const activeTestRuns = computed(
      () => testRuns.value.filter((run) => run.status === "Running").length,
    );
    const completedTestRuns = computed(
      () => testRuns.value.filter((run) => run.status === "Completed").length,
    );
    const readyTestRuns = computed(
      () => testRuns.value.filter((run) => run.status === "Ready").length,
    );
    const overviewRecentRuns = computed(() => testRuns.value.slice(0, 5));
    const overviewAttentionRuns = computed(() =>
      testRuns.value
        .filter((run) => run.failed > 0 || run.blocked > 0)
        .slice(0, 3),
    );
    const attentionRunCount = computed(
      () =>
        testRuns.value.filter((run) => run.failed > 0 || run.blocked > 0)
          .length,
    );
    const executedCheckCount = computed(() =>
      testRuns.value.reduce((total, run) => total + run.executedCases, 0),
    );
    const overallRunPassRate = computed(() => {
      const executed = testRuns.value.reduce(
        (total, run) => total + run.executedCases,
        0,
      );
      const passed = testRuns.value.reduce(
        (total, run) => total + run.passed,
        0,
      );
      return executed
        ? Math.round((passed / executed) * 100)
        : 0;
    });
    const today = computed(() =>
      new Intl.DateTimeFormat(isChinese.value ? "zh-CN" : "en-US", {
        weekday: "long",
        month: "long",
        day: "numeric",
      }).format(new Date()),
    );
    const testCaseColumns = computed(() =>
      TABLE_DEFINITIONS.testCases.columns.map((column) => ({
        ...column,
        label: t(column.label),
      })),
    );
    const testSuiteColumns = computed(() =>
      TABLE_DEFINITIONS.testSuites.columns.map((column) => ({
        ...column,
        label: t(column.label),
      })),
    );
    const filteredTestCases = computed(() => {
      const query = searchQuery.value.trim().toLowerCase();

      return testCases.value.filter((testCase) => {
        const matchesCategory = testCase.category === testCaseCategory.value;
        const matchesSearch =
          !query ||
          [testCase.id, testCase.title, testCase.owner].some(
            (value) => value.toLowerCase().includes(query),
          );
        return matchesCategory && matchesSearch;
      });
    });
    const selectedCategoryTestCases = computed(() =>
      testCases.value.filter(
        (testCase) => testCase.category === testCaseCategory.value,
      ),
    );
    const sortedTestCases = computed(() => {
      if (!testCaseSort.order) {
        return filteredTestCases.value;
      }

      const direction = testCaseSort.order === "ascending" ? 1 : -1;
      const sortValue =
        testCaseSort.prop === "id"
          ? (testCase) => testCaseIdSortValue(testCase.id)
          : (testCase) => updatedSortValue(testCase.updated);

      return [...filteredTestCases.value].sort(
        (first, second) => (sortValue(first) - sortValue(second)) * direction,
      );
    });
    const paginatedTestCases = computed(() => {
      const start = (currentPage.value - 1) * pageSize.value;
      return sortedTestCases.value.slice(start, start + pageSize.value);
    });
    const filteredTestSuites = computed(() => {
      const query = suiteSearchQuery.value.trim().toLowerCase();

      if (!query) {
        return testSuites.value;
      }

      return testSuites.value.filter((suite) =>
        [suite.id, suite.name, suite.description, suite.owner].some((value) =>
          value.toLowerCase().includes(query),
        ),
      );
    });
    const groupedTestCaseCount = computed(
      () => new Set(testSuites.value.flatMap((suite) => suite.caseIds)).size,
    );
    const paginatedTestSuites = computed(() => {
      const start = (suiteCurrentPage.value - 1) * suitePageSize.value;
      return filteredTestSuites.value.slice(
        start,
        start + suitePageSize.value,
      );
    });
    const passedTests = computed(
      () => testCases.value.filter((testCase) => testCase.status === "Passed").length,
    );
    const passRate = computed(() =>
      testCases.value.length
        ? Math.round((passedTests.value / testCases.value.length) * 100)
        : 0,
    );
    const testRunReady = computed(
      () =>
        Boolean(newTestRun.name.trim()) &&
        Boolean(newTestRun.device) &&
        newTestRun.selections.length > 0,
    );
    const selectedDevice = computed(() =>
      devices.value.find((device) => device.id === newTestRun.device),
    );
    const testSelectionOptions = computed(() => {
      if (newTestRun.selectionType === "Test cases") {
        return testCases.value.map((testCase) => ({
          label: `${testCase.id} - ${displayValue(testCase.title)}`,
          value: testCase.id,
        }));
      }

      return testSuites.value.map((suite) => ({
          label: `${displayValue(suite.name)} (${suite.caseIds.length} ${isChinese.value ? "个用例" : "cases"})`,
        value: suite.id,
      }));
    });
    const selectedTestLabels = computed(() => {
      const labelsByValue = new Map(
        testSelectionOptions.value.map((option) => [option.value, option.label]),
      );
      return newTestRun.selections.map(
        (selection) => labelsByValue.get(selection) || selection,
      );
    });
    const selectedRunCaseCount = computed(() => {
      if (newTestRun.selectionType === "Test cases") {
        return newTestRun.selections.length;
      }

      return new Set(
        testSuites.value
          .filter((suite) => newTestRun.selections.includes(suite.id))
          .flatMap((suite) => suite.caseIds),
      ).size;
    });

    watch(
      [searchQuery, testCaseCategory],
      () => {
        currentPage.value = 1;
      },
    );
    watch(pageSize, () => {
      currentPage.value = 1;
    });
    watch(suiteSearchQuery, () => {
      suiteCurrentPage.value = 1;
    });
    watch(suitePageSize, () => {
      suiteCurrentPage.value = 1;
    });
    watch([testRunSearchQuery, testRunStatusFilter], () => {
      testRunCurrentPage.value = 1;
      if (
        filteredTestRuns.value.length &&
        !filteredTestRuns.value.some((run) => run.id === selectedTestRunId.value)
      ) {
        selectedTestRunId.value = filteredTestRuns.value[0].id;
      }
    });
    watch(testRunPageSize, () => {
      testRunCurrentPage.value = 1;
    });
    watch(language, (value) => {
      window.localStorage.setItem("oh-wemby-language", value);
      document.documentElement.lang = value;
      document.title = value === "zh-CN" ? "Oh Wemby - 中文" : "Oh Wemby";
    }, { immediate: true });
    watch(activeView, (view) => {
      if (
        ["Devices", "New Test Run"].includes(view) &&
        appSettings.autoLoadDevices &&
        !devicesLoaded.value
      ) {
        loadDevices();
      }
      if (view === "Settings") {
        loadSettings();
      }
      if (["Test Cases", "New Test Run"].includes(view) && settingsLoaded) {
        loadTestCases();
      }
    }, { immediate: true });
    watch(
      appSettings,
      () => {
        if (!settingsLoaded || applyingSettings) {
          return;
        }
        window.clearTimeout(settingsSaveTimer);
        settingsSaveTimer = window.setTimeout(saveSettings, 450);
      },
      { deep: true },
    );
    watch(
      () => filteredTestCases.value.length,
      (total) => {
        const lastPage = Math.max(1, Math.ceil(total / pageSize.value));
      currentPage.value = Math.min(currentPage.value, lastPage);
      },
    );
    watch(
      () => filteredTestSuites.value.length,
      (total) => {
        const lastPage = Math.max(1, Math.ceil(total / suitePageSize.value));
        suiteCurrentPage.value = Math.min(suiteCurrentPage.value, lastPage);
      },
    );
    watch(
      () => filteredTestRuns.value.length,
      (total) => {
        const lastPage = Math.max(1, Math.ceil(total / testRunPageSize.value));
        testRunCurrentPage.value = Math.min(testRunCurrentPage.value, lastPage);
      },
    );

    function changeSelectedTestCases(selection) {
      selectedTestCases.value = selection;
    }

    function clearSelectedTestCases() {
      selectedTestCases.value = [];
      testCaseTableRef.value?.clearSelection();
    }

    function selectedTestCaseIds() {
      const selectedIds = selectedTestCases.value.map((testCase) => testCase.id);

      if (!selectedIds.length) {
        ElementPlus.ElMessage({
          message: t("Select at least one test case."),
          type: "warning",
        });
      }

      return selectedIds;
    }

    function createSuiteFromSelectedTestCases() {
      const selectedIds = selectedTestCaseIds();

      if (!selectedIds.length) {
        return;
      }

      editingTestSuiteId.value = "";
      Object.assign(newTestSuite, {
        name: "",
        description: "",
        owner: "",
        caseIds: [...new Set(selectedIds)],
      });
      createSuiteDialogVisible.value = true;
    }

    function prepareRunFromSelectedTestCases() {
      const selectedIds = selectedTestCaseIds();

      if (!selectedIds.length) {
        return;
      }

      Object.assign(newTestRun, {
        name: "",
        selectionType: "Test cases",
        selections: [...new Set(selectedIds)],
        notes: "",
      });
      clearSelectedTestCases();
      testRunStarted.value = false;
      selectView("New Test Run");
    }

    function setRouteForView(view, runId = "") {
      const route =
        view === "Test Run Details"
          ? `?view=test-run&run=${encodeURIComponent(runId || selectedTestRunId.value)}`
          : viewRoutes[view] || "";
      const nextUrl = `${window.location.pathname}${route}`;
      const currentUrl = `${window.location.pathname}${window.location.search}`;

      if (nextUrl !== currentUrl) {
        window.history.pushState({}, "", nextUrl);
      }
    }

    function selectView(view, options = {}) {
      if (activeView.value === "Test Cases" && view !== "Test Cases") {
        clearSelectedTestCases();
      }
      if (options.runId) {
        selectedTestRunId.value = options.runId;
      }
      if (!options.skipHistory) {
        setRouteForView(view, options.runId);
      }
      activeView.value = view;
      if (!["Overview", "Devices", "Test Cases", "Test Suites", "New Test Run", "Tasks", "Settings"].includes(view)) {
        ElementPlus.ElMessage({
          message: isChinese.value
            ? `${t(view)}已准备好进行下一步开发。`
            : `${view} is ready for the next development step.`,
          type: "info",
        });
      }
    }

    function applySettings(settings) {
      applyingSettings = true;
      const nextSettings = {
        ...DEFAULT_APP_SETTINGS,
        ...settings,
      };
      Object.assign(appSettings, {
        ...nextSettings,
        deviceRefreshSeconds: Number(nextSettings.deviceRefreshSeconds) || DEFAULT_APP_SETTINGS.deviceRefreshSeconds,
        tablePageSize: Number(nextSettings.tablePageSize) || DEFAULT_APP_SETTINGS.tablePageSize,
        autoLoadDevices: Boolean(nextSettings.autoLoadDevices),
      });
      pageSize.value = appSettings.tablePageSize;
      testRunPageSize.value = appSettings.tablePageSize;
      suitePageSize.value = appSettings.tablePageSize;
      window.setTimeout(() => {
        applyingSettings = false;
      }, 0);
    }

    async function loadSettings() {
      settingsLoading.value = true;
      settingsError.value = "";

      try {
        const response = await fetch("/api/settings", { cache: "no-store" });
        const result = await response.json();
        if (!response.ok) {
          throw new Error(result.error || `The settings service returned HTTP ${response.status}.`);
        }
        applySettings(result.settings || {});
        settingsLoaded = true;
        await loadTestCases();
      } catch (error) {
        settingsError.value =
          error instanceof Error ? error.message : "Unable to load settings.";
      } finally {
        settingsLoading.value = false;
      }
    }

    async function saveSettings() {
      settingsSaving.value = true;
      settingsError.value = "";

      try {
        const response = await fetch("/api/settings", {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ settings: appSettings }),
        });
        const result = await response.json();
        if (!response.ok) {
          throw new Error(result.error || `The settings service returned HTTP ${response.status}.`);
        }
        applySettings(result.settings || {});
        await loadTestCases();
        settingsSavedAt.value = new Intl.DateTimeFormat(
          isChinese.value ? "zh-CN" : "en-US",
          { hour: "numeric", minute: "2-digit", second: "2-digit" },
        ).format(new Date());
      } catch (error) {
        settingsError.value =
          error instanceof Error ? error.message : "Unable to save settings.";
      } finally {
        settingsSaving.value = false;
      }
    }

    function resetSettings() {
      applySettings(DEFAULT_APP_SETTINGS);
      settingsLoaded = true;
      saveSettings();
      ElementPlus.ElMessage({
        message: t("Settings restored to defaults."),
        type: "success",
      });
    }

    async function loadDevices() {
      devicesLoading.value = true;
      devicesError.value = "";

      try {
        const response = await fetch("/api/devices", { cache: "no-store" });
        if (!response.ok) {
          throw new Error(`The device service returned HTTP ${response.status}.`);
        }
        const result = await response.json();
        devices.value = Array.isArray(result.devices) ? result.devices : [];
        devicesError.value = result.error || "";
        devicesLoaded.value = true;

        if (
          newTestRun.device &&
          !devices.value.some((device) => device.id === newTestRun.device)
        ) {
          newTestRun.device = "";
        }
      } catch (error) {
        devices.value = [];
        devicesError.value =
          error instanceof Error ? error.message : "Unable to search for devices.";
      } finally {
        devicesLoading.value = false;
      }
    }

    async function loadTestCases() {
      testCasesLoading.value = true;
      testCasesError.value = "";

      try {
        const response = await fetch("/api/test-cases", { cache: "no-store" });
        const result = await response.json();
        if (!response.ok) {
          throw new Error(
            result.error ||
              `The test case service returned HTTP ${response.status}.`,
          );
        }
        testCases.value = Array.isArray(result.testCases)
          ? result.testCases
          : [];
        testCasesError.value = result.error || "";
        newTestRun.selections = newTestRun.selections.filter((selection) =>
          testCases.value.some((testCase) => testCase.id === selection),
        );
      } catch (error) {
        testCases.value = [];
        testCasesError.value =
          error instanceof Error ? error.message : "Unable to load test cases.";
      } finally {
        testCasesLoading.value = false;
      }
    }

    function addTask() {
      selectView("New Test Run");
    }

    function selectTestRun(run) {
      selectView("Test Run Details", { runId: run.id });
    }

    function backToTestRuns() {
      selectView("Tasks");
    }

    function runStatusType(status) {
      return {
        Completed: "success",
        Running: "primary",
        Blocked: "danger",
        Ready: "info",
      }[status] || "info";
    }

    function syncViewFromLocation() {
      const params = new URLSearchParams(window.location.search);
      const nextView = requestedViews[params.get("view")] || "Overview";
      const runId = params.get("run");
      if (runId && testRuns.value.some((run) => run.id === runId)) {
        selectedTestRunId.value = runId;
      }
      selectView(nextView, { skipHistory: true });
    }

    onMounted(() => {
      loadSettings();
      window.addEventListener("popstate", syncViewFromLocation);
    });
    onBeforeUnmount(() => {
      window.clearTimeout(settingsSaveTimer);
      window.removeEventListener("popstate", syncViewFromLocation);
    });

    function changeTestCaseSort({ order, prop }) {
      if (!["id", "updated"].includes(prop)) {
        return;
      }

      Object.assign(testCaseSort, { prop, order });
    }

    function openCreateDialog() {
      editingTestCaseId.value = "";
      Object.assign(newTestCase, {
        title: "",
        owner: "",
      });
      createDialogVisible.value = true;
    }

    function openCreateSuiteDialog() {
      editingTestSuiteId.value = "";
      Object.assign(newTestSuite, {
        name: "",
        description: "",
        owner: "",
        caseIds: [],
      });
      createSuiteDialogVisible.value = true;
    }

    function editTestCase(testCase) {
      editingTestCaseId.value = testCase.id;
      Object.assign(newTestCase, {
        title: testCase.title,
        owner: testCase.owner === "Unassigned" ? "" : testCase.owner,
      });
      createDialogVisible.value = true;
    }

    function saveTestCase() {
      if (!newTestCase.title.trim()) {
        ElementPlus.ElMessage({
          message: t("Enter a test case title."),
          type: "warning",
        });
        return;
      }

      if (editingTestCaseId.value) {
        const testCase = testCases.value.find(
          (item) => item.id === editingTestCaseId.value,
        );
        Object.assign(testCase, {
          title: newTestCase.title.trim(),
          owner: newTestCase.owner.trim() || "Unassigned",
          updated: "Just now",
        });
        createDialogVisible.value = false;
        editingTestCaseId.value = "";
        ElementPlus.ElMessage({
          message: t("Test case updated."),
          type: "success",
        });
        return;
      }

      const nextId =
        Math.max(
          0,
          ...testCases.value.map((testCase) => Number(testCase.id.slice(3))),
        ) +
        1;
      testCases.value.unshift({
        id: `TC-${nextId}`,
        title: newTestCase.title.trim(),
        category: "Customized",
        status: "Not run",
        owner: newTestCase.owner.trim() || "Unassigned",
        updated: "Just now",
      });
      testCaseCategory.value = "Customized";
      currentPage.value = 1;
      Object.assign(newTestCase, {
        title: "",
        owner: "",
      });
      createDialogVisible.value = false;
      ElementPlus.ElMessage({
        message: t("Test case created."),
        type: "success",
      });
    }

    function runTestCase(testCase) {
      testCase.status = "Passed";
      testCase.updated = "Just now";
      ElementPlus.ElMessage({
        message: `${testCase.id} completed successfully.`,
        type: "success",
      });
    }

    function editTestSuite(suite) {
      editingTestSuiteId.value = suite.id;
      Object.assign(newTestSuite, {
        name: suite.name,
        description: suite.description,
        owner: suite.owner === "Unassigned" ? "" : suite.owner,
        caseIds: [...suite.caseIds],
      });
      createSuiteDialogVisible.value = true;
    }

    function saveTestSuite() {
      if (!newTestSuite.name.trim()) {
        ElementPlus.ElMessage({
          message: t("Enter a test suite name."),
          type: "warning",
        });
        return;
      }

      if (!newTestSuite.caseIds.length) {
        ElementPlus.ElMessage({
          message: t("Select at least one test case."),
          type: "warning",
        });
        return;
      }

      if (editingTestSuiteId.value) {
        const suite = testSuites.value.find(
          (item) => item.id === editingTestSuiteId.value,
        );
        Object.assign(suite, {
          name: newTestSuite.name.trim(),
          description:
            newTestSuite.description.trim() || "Reusable test case group.",
          owner: newTestSuite.owner.trim() || "Unassigned",
          caseIds: [...newTestSuite.caseIds],
          updated: "Just now",
        });
        createSuiteDialogVisible.value = false;
        editingTestSuiteId.value = "";
        ElementPlus.ElMessage({
          message: t("Test suite updated."),
          type: "success",
        });
        return;
      }

      const nextId =
        Math.max(
          ...testSuites.value.map((suite) => Number(suite.id.slice(3))),
        ) + 1;
      testSuites.value.unshift({
        id: `TS-${nextId}`,
        name: newTestSuite.name.trim(),
        description:
          newTestSuite.description.trim() || "Reusable test case group.",
        owner: newTestSuite.owner.trim() || "Unassigned",
        caseIds: [...newTestSuite.caseIds],
        updated: "Just now",
      });
      suiteCurrentPage.value = 1;
      Object.assign(newTestSuite, {
        name: "",
        description: "",
        owner: "",
        caseIds: [],
      });
      createSuiteDialogVisible.value = false;
      ElementPlus.ElMessage({
        message: t("Test suite created."),
        type: "success",
      });
    }

    function configureSuiteRun(suite) {
      Object.assign(newTestRun, {
        name: `${suite.name} run`,
        selectionType: "Test cases",
        selections: [...suite.caseIds],
        notes: "",
      });
      testRunStarted.value = false;
      selectView("New Test Run");
    }

    async function startTestRun() {
      if (!newTestRun.name.trim()) {
        ElementPlus.ElMessage({
          message: t("Enter a test run name."),
          type: "warning",
        });
        return;
      }

      if (!newTestRun.device) {
        ElementPlus.ElMessage({
          message: t("Select a device."),
          type: "warning",
        });
        return;
      }

      if (!newTestRun.selections.length) {
        ElementPlus.ElMessage({
          message: isChinese.value
            ? `请至少选择一个${t(newTestRun.selectionType)}。`
            : `Select at least one ${newTestRun.selectionType.toLowerCase().slice(0, -1)}.`,
          type: "warning",
        });
        return;
      }

      testRunStarting.value = true;
      testRunError.value = "";

      try {
        const response = await fetch("/api/test-runs", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            testCases: newTestRun.selections,
            inspectionMode: newTestRun.inspectionMode,
          }),
        });
        const result = await response.json();
        if (!response.ok) {
          throw new Error(
            result.error ||
              `The test run service returned HTTP ${response.status}.`,
          );
        }
        testRunExecution.value = result;
        testRunStarted.value = true;
        ElementPlus.ElMessage({
          message: isChinese.value
            ? `${newTestRun.name}已启动 ${result.started.length} 个测试用例。`
            : `${newTestRun.name} started ${result.started.length} test cases.`,
          type: "success",
        });
      } catch (error) {
        testRunError.value =
          error instanceof Error ? error.message : "Unable to start the test run.";
        ElementPlus.ElMessage({
          message: t("Test run failed"),
          type: "error",
        });
      } finally {
        testRunStarting.value = false;
      }
    }

    function tableValue(row, column) {
      if (column.renderer === "count") {
        return row[column.field].length;
      }
      return displayValue(row[column.field]);
    }

    return {
      activeView,
      appSettings,
      devices,
      devicesError,
      devicesLoading,
      language,
      loadDevices,
      loadSettings,
      loadTestCases,
      resetSettings,
      isChinese,
      t,
      displayValue,
      addTask,
      activeTestRuns,
      completedTestRuns,
      createDialogVisible,
      createSuiteDialogVisible,
      configureSuiteRun,
      currentPage,
      editTestCase,
      editTestSuite,
      editingTestCaseId,
      editingTestSuiteId,
      filteredTestCases,
      filteredTestSuites,
      filteredTestRuns,
      groupedTestCaseCount,
      newTestCase,
      newTestSuite,
      openCreateDialog,
      openCreateSuiteDialog,
      pageSize,
      paginatedTestCases,
      paginatedTestSuites,
      passedTests,
      passRate,
      overallRunPassRate,
      readyTestRuns,
      overviewRecentRuns,
      overviewAttentionRuns,
      attentionRunCount,
      executedCheckCount,
      paginatedTestRuns,
      selectedTestRun,
      selectedTestRunId,
      selectTestRun,
      backToTestRuns,
      runStatusType,
      testRunCurrentPage,
      testRunPageSize,
      testRunSearchQuery,
      testRunStatusFilter,
      testRuns,
      runTestCase,
      saveSettings,
      saveTestCase,
      saveTestSuite,
      searchQuery,
      selectView,
      selectedRunCaseCount,
      selectedDevice,
      tableValue,
      startTestRun,
      changeSelectedTestCases,
      clearSelectedTestCases,
      createSuiteFromSelectedTestCases,
      prepareRunFromSelectedTestCases,
      tasks,
      testCases,
      testCaseTableRef,
      testCaseCategory,
      testCaseColumns,
      selectedCategoryTestCases,
      selectedTestCases,
      testSuites,
      testSuiteColumns,
      suiteSearchQuery,
      suiteCurrentPage,
      suitePageSize,
      selectedTestLabels,
      testSelectionOptions,
      testRunReady,
      testRunStarted,
      testRunStarting,
      testRunError,
      testRunExecution,
      testCasesLoading,
      testCasesError,
      settingsError,
      settingsLoading,
      settingsSaving,
      settingsSavedAt,
      today,
      changeTestCaseSort,
      deviceOptions: devices,
      newTestRun,
    };
  },
  template: `
    <div class="app-shell">
      <aside class="sidebar">
        <a class="brand" href="#" aria-label="Oh Wemby home">
          <span class="brand-mark">W</span>
          <span>{{ appSettings.projectName }}</span>
        </a>

        <nav class="nav-list" :aria-label="t('Primary navigation')">
          <button
            v-for="view in ['Overview', 'Devices', 'Test Cases', 'Tasks']"
            :key="view"
            class="nav-item"
            :class="{
              active:
                activeView === view ||
                (view === 'Tasks' && activeView === 'Test Run Details')
            }"
            type="button"
            @click="selectView(view)"
          >
            <span class="nav-dot"></span>
            {{ t(view === 'Tasks' ? 'Test runs' : view) }}
          </button>
        </nav>

        <div class="sidebar-tools">
          <el-select
            v-model="language"
            class="language-select"
            aria-label="Display language"
          >
            <el-option label="English" value="en" />
            <el-option label="中文" value="zh-CN" />
          </el-select>
          <el-tooltip :content="t('Settings')" placement="top">
            <button
              class="settings-entry"
              :class="{ active: activeView === 'Settings' }"
              type="button"
              :aria-label="t('Settings')"
              @click="selectView('Settings')"
            >
              ⚙
            </button>
          </el-tooltip>
        </div>
      </aside>

      <main class="main-content">
        <header class="topbar">
          <div>
            <p class="eyebrow">{{ today }}</p>
            <h1>
              {{ t(activeView === 'Tasks'
                ? 'Test runs'
                : activeView === 'Test Run Details'
                  ? 'Test run details'
                  : activeView) }}
            </h1>
          </div>
          <el-button
            v-if="activeView === 'Test Cases'"
            type="primary"
            round
            :loading="testCasesLoading"
            @click="loadTestCases"
          >
            {{ t('Reload test cases') }}
          </el-button>
          <el-button
            v-else-if="activeView === 'Test Suites'"
            type="primary"
            round
            @click="openCreateSuiteDialog"
          >
            + {{ t('New test suite') }}
          </el-button>
          <el-button
            v-else-if="activeView === 'New Test Run'"
            type="primary"
            round
            :loading="testRunStarting"
            @click="startTestRun"
          >
            {{ t('Start test run') }}
          </el-button>
          <el-button
            v-else-if="activeView === 'Tasks'"
            type="primary"
            round
            @click="selectView('New Test Run')"
          >
            + {{ t('New test run') }}
          </el-button>
          <el-button
            v-else-if="activeView === 'Test Run Details'"
            plain
            round
            @click="backToTestRuns"
          >
            ← {{ t('Back to test runs') }}
          </el-button>
        </header>

        <template v-if="activeView === 'Overview'">
          <section class="overview-hero">
            <div class="overview-hero-copy">
              <p class="eyebrow">{{ t('Release quality overview') }}</p>
              <h2>{{ t('See what’s ready. Fix what’s not.') }}</h2>
              <p>
                {{ t('Track tests, review issues, and start new runs.') }}
              </p>
              <div class="overview-actions">
                <el-button type="primary" size="large" @click="addTask">
                  + {{ t('New test run') }}
                </el-button>
                <el-button size="large" @click="selectView('Tasks')">
                  {{ t('View all runs') }}
                </el-button>
              </div>
            </div>

            <aside class="confidence-card" :aria-label="t('Release confidence')">
              <div class="confidence-heading">
                <span>{{ displayValue(appSettings.releaseName) }} · {{ appSettings.defaultEnvironment }}</span>
                <el-tag type="success" effect="light" round>{{ t('On track') }}</el-tag>
              </div>
              <div class="confidence-score">
                <strong>{{ overallRunPassRate }}%</strong>
                <span>{{ t('Release confidence') }}</span>
              </div>
              <el-progress
                :percentage="overallRunPassRate"
                :show-text="false"
                :stroke-width="8"
                color="#58b98b"
              />
              <small>
                {{ isChinese
                  ? '基于 ' + executedCheckCount + ' 项已执行的检查'
                  : 'Based on ' + executedCheckCount + ' executed checks' }}
              </small>
            </aside>
          </section>

          <section class="overview-metrics" :aria-label="t('Release quality overview')">
            <article class="overview-metric">
              <span class="metric-symbol metric-symbol-purple">↗</span>
              <div>
                <span>{{ t('Active runs') }}</span>
                <strong>{{ activeTestRuns }}</strong>
                <small>{{ t('Running now') }}</small>
              </div>
            </article>
            <article class="overview-metric">
              <span class="metric-symbol metric-symbol-mint">✓</span>
              <div>
                <span>{{ t('Completed runs') }}</span>
                <strong>{{ completedTestRuns }}</strong>
                <small>{{ isChinese ? '共 ' + testRuns.length + ' 次运行' : 'of ' + testRuns.length + ' total runs' }}</small>
              </div>
            </article>
            <article class="overview-metric">
              <span class="metric-symbol metric-symbol-amber">!</span>
              <div>
                <span>{{ t('Runs to review') }}</span>
                <strong>{{ attentionRunCount }}</strong>
                <small>{{ t('Failed or blocked checks') }}</small>
              </div>
            </article>
            <article class="overview-metric">
              <span class="metric-symbol metric-symbol-neutral">○</span>
              <div>
                <span>{{ t('Ready to run') }}</span>
                <strong>{{ readyTestRuns }}</strong>
                <small>{{ t('Awaiting execution') }}</small>
              </div>
            </article>
          </section>

          <section class="overview-content">
            <article class="overview-panel recent-runs-panel">
              <div class="overview-panel-heading">
                <div>
                  <p class="eyebrow">{{ t('Recent validation') }}</p>
                  <h3>{{ t('Recent test runs') }}</h3>
                  <p>{{ t('Latest test runs across the release.') }}</p>
                </div>
                <el-button text type="primary" @click="selectView('Tasks')">
                  {{ t('View all runs') }} →
                </el-button>
              </div>

              <div class="overview-run-list">
                <button
                  v-for="run in overviewRecentRuns"
                  :key="run.id"
                  class="overview-run-row"
                  type="button"
                  @click="selectTestRun(run)"
                >
                  <span class="run-state-mark" :class="run.status.toLowerCase()"></span>
                  <span class="overview-run-copy">
                    <strong>{{ displayValue(run.title) }}</strong>
                    <small>{{ run.id }} · {{ run.device }} · {{ displayValue(run.updated) }}</small>
                  </span>
                  <span class="overview-run-result">
                    <strong>{{ run.passed }}/{{ run.executedCases }}</strong>
                    <small>{{ t('Passed') }}</small>
                  </span>
                  <el-tag
                    :type="runStatusType(run.status)"
                    size="small"
                    effect="light"
                    round
                  >
                    {{ t(run.status) }}
                  </el-tag>
                </button>
              </div>
            </article>

            <aside class="overview-panel attention-panel">
              <div class="overview-panel-heading">
                <div>
                  <p class="eyebrow">{{ t('Attention needed') }}</p>
                  <h3>{{ t('Runs to review') }}</h3>
                  <p>{{ t('Review these runs before release approval.') }}</p>
                </div>
              </div>

              <div v-if="overviewAttentionRuns.length" class="attention-list">
                <button
                  v-for="run in overviewAttentionRuns"
                  :key="run.id"
                  type="button"
                  @click="selectTestRun(run)"
                >
                  <span class="attention-icon">!</span>
                  <span>
                    <strong>{{ displayValue(run.title) }}</strong>
                    <small>
                      {{ run.failed }} {{ t('Failed').toLowerCase() }} ·
                      {{ run.blocked }} {{ t('Blocked').toLowerCase() }}
                    </small>
                  </span>
                  <span aria-hidden="true">→</span>
                </button>
              </div>
              <div v-else class="attention-empty">
                <span>✓</span>
                <strong>{{ t('No runs need attention') }}</strong>
                <small>{{ t('All recorded checks are clear.') }}</small>
              </div>
            </aside>
          </section>
        </template>

        <template v-else-if="activeView === 'Tasks'">
          <section class="test-runs-intro">
            <div>
              <p class="eyebrow">{{ t('Quality assurance workflow') }}</p>
              <h2>{{ t('Test runs') }}</h2>
              <p>{{ t('Monitor active runs, review outcomes, and keep release validation moving.') }}</p>
            </div>
            <div class="run-overview-stats" :aria-label="t('Test runs')">
              <div>
                <span>{{ t('Active runs') }}</span>
                <strong>{{ activeTestRuns }}</strong>
              </div>
              <div>
                <span>{{ t('Completed runs') }}</span>
                <strong>{{ completedTestRuns }}</strong>
              </div>
              <div>
                <span>{{ t('Overall pass rate') }}</span>
                <strong>{{ overallRunPassRate }}%</strong>
              </div>
            </div>
          </section>

          <section class="test-runs-workspace">
            <div class="run-list-heading">
              <div>
                <h3>{{ t('Test run history') }}</h3>
                <p>{{ isChinese ? '共 ' + filteredTestRuns.length + ' 次运行' : filteredTestRuns.length + ' runs' }}</p>
              </div>
              <div class="test-run-toolbar">
                <el-input
                  v-model="testRunSearchQuery"
                  clearable
                  :placeholder="t('Search by run, ID, device, suite, or owner')"
                  aria-label="Search test runs"
                />
                <el-select
                  v-model="testRunStatusFilter"
                  clearable
                  :placeholder="t('All run statuses')"
                  aria-label="Filter test runs by status"
                >
                  <el-option
                    v-for="status in ['Running', 'Ready', 'Blocked', 'Completed']"
                    :key="status"
                    :label="t(status)"
                    :value="status"
                  />
                </el-select>
              </div>
            </div>

            <el-table
              :data="paginatedTestRuns"
              class="test-runs-table"
              border
              stripe
              :empty-text="t('No test runs match these filters')"
              row-key="id"
              @row-click="selectTestRun"
            >
              <el-table-column :label="t('Run')" min-width="290">
                <template #default="{ row }">
                  <button class="run-name-cell" type="button" @click.stop="selectTestRun(row)">
                    <span>{{ row.id }} · {{ displayValue(row.suite) }}</span>
                    <strong>{{ displayValue(row.title) }}</strong>
                  </button>
                </template>
              </el-table-column>
              <el-table-column :label="t('Status')" width="120">
                <template #default="{ row }">
                  <el-tag :type="runStatusType(row.status)" effect="light" round>
                    {{ t(row.status) }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="device" :label="t('Device')" width="145" />
              <el-table-column prop="owner" :label="t('Owner')" width="135" />
              <el-table-column :label="t('Results')" width="140">
                <template #default="{ row }">
                  <div class="run-result-cell">
                    <span class="passed" :title="t('Passed')">{{ row.passed }}</span>
                    <span class="failed" :title="t('Failed')">{{ row.failed }}</span>
                    <span class="blocked" :title="t('Blocked')">{{ row.blocked }}</span>
                  </div>
                </template>
              </el-table-column>
              <el-table-column :label="t('Progress')" width="155">
                <template #default="{ row }">
                  <div class="run-progress-cell">
                    <el-progress
                      :percentage="row.progress"
                      :show-text="false"
                      :stroke-width="7"
                      color="#5c6cff"
                    />
                    <span>{{ row.executedCases }}/{{ row.totalCases }}</span>
                  </div>
                </template>
              </el-table-column>
              <el-table-column :label="t('Updated')" width="105">
                <template #default="{ row }">{{ displayValue(row.updated) }}</template>
              </el-table-column>
              <el-table-column :label="t('Actions')" width="90" fixed="right">
                <template #default="{ row }">
                  <el-button plain size="small" @click.stop="selectTestRun(row)">
                    {{ t('View') }}
                  </el-button>
                </template>
              </el-table-column>
            </el-table>

            <div class="table-pagination">
              <el-pagination
                v-model:current-page="testRunCurrentPage"
                v-model:page-size="testRunPageSize"
                :page-sizes="[10, 20, 50, 100]"
                :total="filteredTestRuns.length"
                layout="total, sizes, prev, pager, next"
              />
            </div>
          </section>
        </template>

        <template v-else-if="activeView === 'Test Run Details'">
          <section v-if="selectedTestRun" class="test-run-detail">
            <article class="run-detail-main">
              <div class="run-detail-title">
                <div>
                  <p class="eyebrow">{{ selectedTestRun.id }} · {{ t('Run details') }}</p>
                  <h3>{{ displayValue(selectedTestRun.title) }}</h3>
                </div>
                <el-tag
                  :type="runStatusType(selectedTestRun.status)"
                  effect="light"
                  round
                >
                  {{ t(selectedTestRun.status) }}
                </el-tag>
              </div>
              <p class="run-description">{{ displayValue(selectedTestRun.description) }}</p>

              <dl class="run-metadata">
                <div>
                  <dt>{{ t('Device') }}</dt>
                  <dd>{{ selectedTestRun.device }}</dd>
                </div>
                <div>
                  <dt>{{ t('Test suite') }}</dt>
                  <dd>{{ displayValue(selectedTestRun.suite) }}</dd>
                </div>
                <div>
                  <dt>{{ t('Environment') }}</dt>
                  <dd>{{ t(selectedTestRun.environment) }}</dd>
                </div>
                <div>
                  <dt>{{ t('Owner') }}</dt>
                  <dd>{{ selectedTestRun.owner }}</dd>
                </div>
              </dl>

              <div class="execution-progress">
                <div>
                  <span>{{ t('Execution summary') }}</span>
                  <strong>{{ selectedTestRun.progress }}%</strong>
                </div>
                <el-progress
                  :percentage="selectedTestRun.progress"
                  :show-text="false"
                  :stroke-width="8"
                  color="#5c6cff"
                />
              </div>
            </article>

            <aside class="run-result-report">
              <div class="section-heading">
                <div>
                  <p class="eyebrow">{{ t('Results') }}</p>
                  <h3>{{ t('Execution summary') }}</h3>
                </div>
                <span class="run-duration">{{ selectedTestRun.duration }}</span>
              </div>
              <div class="result-summary-grid">
                <div>
                  <span>{{ t('Total') }}</span>
                  <strong>{{ selectedTestRun.totalCases }}</strong>
                </div>
                <div class="result-passed">
                  <span>{{ t('Passed') }}</span>
                  <strong>{{ selectedTestRun.passed }}</strong>
                </div>
                <div class="result-failed">
                  <span>{{ t('Failed') }}</span>
                  <strong>{{ selectedTestRun.failed }}</strong>
                </div>
                <div class="result-blocked">
                  <span>{{ t('Blocked') }}</span>
                  <strong>{{ selectedTestRun.blocked }}</strong>
                </div>
              </div>
              <p class="run-result-note">
                {{ t(selectedTestRun.status === 'Blocked'
                  ? 'One blocked case requires a restored test account before execution can continue.'
                  : selectedTestRun.failed || selectedTestRun.blocked
                    ? 'Review the recorded failures before closing this run.'
                    : 'Completed without blocking issues.') }}
              </p>
              <el-button plain @click="selectView('Test Cases')">
                {{ t('View test cases') }}
              </el-button>
            </aside>
          </section>

          <el-empty v-else :description="t('No test runs match these filters')">
            <el-button type="primary" @click="backToTestRuns">
              {{ t('Back to test runs') }}
            </el-button>
          </el-empty>
        </template>

        <template v-else-if="activeView === 'Devices'">
          <section class="devices-hero">
            <div>
              <p class="eyebrow">{{ t('Test devices') }}</p>
              <h2>{{ t('See every device ready for testing.') }}</h2>
              <p>{{ t('Find devices connected through HDC and confirm each target before starting a run.') }}</p>
            </div>
            <el-button type="primary" :loading="devicesLoading" @click="loadDevices">
              {{ t('Refresh devices') }}
            </el-button>
          </section>

          <section class="devices-workspace" v-loading="devicesLoading">
            <el-result
              v-if="devicesError"
              icon="error"
              :title="t('Device search failed')"
              :sub-title="t('Try again after resolving the HDC issue shown below.')"
            >
              <template #extra>
                <div class="device-error-detail">{{ devicesError }}</div>
                <el-button type="primary" @click="loadDevices">
                  {{ t('Refresh devices') }}
                </el-button>
              </template>
            </el-result>

            <el-empty
              v-else-if="!devicesLoading && !devices.length"
              :description="t('No connected devices found')"
            >
              <p class="device-empty-help">
                {{ t('Connect a device, confirm that HDC can access it, and refresh the list.') }}
              </p>
            </el-empty>

            <div v-else class="device-list">
              <article v-for="device in devices" :key="device.id" class="device-card">
                <span class="device-card-icon">D</span>
                <div class="device-card-content">
                  <div class="device-card-heading">
                    <div>
                      <strong>{{ device.name || device.model || device.id }}</strong>
                      <span>{{ device.id }}</span>
                    </div>
                    <el-tag type="success" effect="light">
                      {{ t('Connected') }}
                    </el-tag>
                  </div>
                  <dl class="device-details">
                    <div>
                      <dt>{{ t('Model') }}</dt>
                      <dd>{{ device.model || t('Unknown') }}</dd>
                    </div>
                    <div>
                      <dt>{{ t('OS version') }}</dt>
                      <dd>{{ device.osVersion || t('Unknown') }}</dd>
                    </div>
                    <div>
                      <dt>{{ t('Type') }}</dt>
                      <dd>{{ device.deviceType || t('Unknown') }}</dd>
                    </div>
                  </dl>
                </div>
              </article>
            </div>
          </section>
        </template>

        <template v-else-if="activeView === 'Test Cases'">
          <section class="test-summary" aria-label="Test suite summary">
            <article class="summary-intro">
              <div>
                <p class="eyebrow">{{ t('Release 2.4 regression') }}</p>
                <h2>{{ t('Quality at a glance') }}</h2>
                <p>{{ t('Track coverage, review failures, and keep every release decision tied to a clear test result.') }}</p>
              </div>
              <div class="pass-ring">
                <el-progress
                  type="circle"
                  :percentage="passRate"
                  :width="116"
                  :stroke-width="9"
                  color="#63d3a1"
                />
                <span>{{ t('Pass rate') }}</span>
              </div>
            </article>

            <article class="test-metric">
              <span class="metric-icon metric-purple">✓</span>
              <div>
                <strong>{{ passedTests }}</strong>
                <span>{{ t('Passing') }}</span>
              </div>
              <small>{{ isChinese ? '共 ' + testCases.length + ' 项' : 'of ' + testCases.length + ' total' }}</small>
            </article>
            <article class="test-metric">
              <span class="metric-icon metric-coral">!</span>
              <div>
                <strong>{{ testCases.length - passedTests }}</strong>
                <span>{{ t('Needs attention') }}</span>
              </div>
              <small>{{ t('failed, blocked or pending') }}</small>
            </article>
          </section>

          <section class="test-workspace">
            <el-alert
              v-if="testCasesError"
              class="test-library-alert"
              type="warning"
              :title="t('Test case library unavailable')"
              :description="testCasesError"
              show-icon
              :closable="false"
            >
              <template #default>
                <el-button size="small" plain @click="loadTestCases">
                  {{ t('Reload test cases') }}
                </el-button>
              </template>
            </el-alert>
            <div class="section-heading test-list-heading">
              <div>
                <p class="eyebrow">{{ t('Test library') }}</p>
                <h3>
                  {{ testCaseCategory === 'Standard'
                    ? t('Standard test cases')
                    : t('Customized test cases') }}
                </h3>
              </div>
              <span class="result-count">
                {{ isChinese
                  ? filteredTestCases.length + ' / ' + selectedCategoryTestCases.length + ' 个用例'
                  : filteredTestCases.length + ' of ' + selectedCategoryTestCases.length + ' cases' }}
              </span>
            </div>

            <div class="test-category-switch">
              <el-radio-group
                v-model="testCaseCategory"
                class="test-selection-type"
                aria-label="Test case category"
              >
                <el-radio-button value="Standard">
                  {{ t('Standard') }}
                </el-radio-button>
                <el-radio-button value="Customized" disabled>
                  {{ t('Customized') }}
                </el-radio-button>
              </el-radio-group>
              <span>{{ t('Contact an administrator to access customized test cases.') }}</span>
            </div>

            <div class="test-toolbar">
              <el-input
                v-model="searchQuery"
                class="test-search"
                clearable
                :placeholder="t('Search by ID, title, or owner')"
              />
            </div>

            <div class="bulk-action-bar" v-if="selectedTestCases.length">
              <div>
                <strong>{{ selectedTestCases.length }}</strong>
                <span>
                  {{ isChinese
                    ? '个测试用例已选择'
                    : (selectedTestCases.length === 1 ? 'test case selected' : 'test cases selected') }}
                </span>
              </div>
              <div class="bulk-actions">
                <el-button
                  size="small"
                  @click="createSuiteFromSelectedTestCases"
                >
                  {{ t('Add to Test Suite') }}
                </el-button>
                <el-button
                  size="small"
                  type="primary"
                  plain
                  @click="prepareRunFromSelectedTestCases"
                >
                  {{ t('Create Test Run') }}
                </el-button>
                <el-button size="small" text @click="clearSelectedTestCases">
                  {{ t('Clear') }}
                </el-button>
              </div>
            </div>

            <el-table
              ref="testCaseTableRef"
              v-loading="testCasesLoading"
              :data="paginatedTestCases"
              class="test-table"
              border
              stripe
              row-key="id"
              :default-sort="{ prop: 'updated', order: 'ascending' }"
              :empty-text="t('No test cases match these filters')"
              @selection-change="changeSelectedTestCases"
              @sort-change="changeTestCaseSort"
            >
              <el-table-column
                type="selection"
                width="48"
                reserve-selection
              />
              <el-table-column
                v-for="column in testCaseColumns"
                :key="column.field"
                :prop="column.renderer === 'actions' ? undefined : column.field"
                :label="column.label"
                :width="column.width"
                :min-width="column.minWidth"
                :fixed="column.fixed"
                :sortable="column.sortable"
                :show-overflow-tooltip="column.showOverflowTooltip"
              >
                <template #default="{ row }">
                  <div
                    v-if="column.renderer === 'actions'"
                    class="table-row-actions"
                  >
                    <el-tooltip
                      :content="t('Contact an administrator to edit test cases.')"
                      placement="top"
                    >
                      <span>
                        <el-button plain size="small" disabled>
                          {{ t('Edit') }}
                        </el-button>
                      </span>
                    </el-tooltip>
                  </div>
                  <template v-else>{{ tableValue(row, column) }}</template>
                </template>
              </el-table-column>
            </el-table>

            <div class="table-pagination">
              <el-pagination
                v-model:current-page="currentPage"
                v-model:page-size="pageSize"
                :page-sizes="[10, 20, 50, 100]"
                :total="filteredTestCases.length"
                layout="total, sizes, prev, pager, next"
              />
            </div>
          </section>
        </template>

        <template v-else-if="activeView === 'Test Suites'">
          <section class="suite-hero">
            <div>
              <p class="eyebrow">{{ t('Reusable test case groups') }}</p>
              <h2>{{ t('Build once, run together.') }}</h2>
              <p>{{ t('Organize related test cases into suites, then select one or more suites when starting a focused test run.') }}</p>
            </div>
            <div class="suite-hero-stats" aria-label="Test suite summary">
              <div>
                <strong>{{ testSuites.length }}</strong>
                <span>{{ t('Test suites') }}</span>
              </div>
              <div>
                <strong>{{ groupedTestCaseCount }}</strong>
                <span>{{ t('Grouped cases') }}</span>
              </div>
            </div>
          </section>

          <section class="suite-workspace">
            <div class="section-heading suite-list-heading">
              <div>
                <p class="eyebrow">{{ t('Suite library') }}</p>
                <h3>{{ t('All test suites') }}</h3>
              </div>
              <span class="result-count">
                {{ isChinese
                  ? filteredTestSuites.length + ' / ' + testSuites.length + ' 个套件'
                  : filteredTestSuites.length + ' of ' + testSuites.length + ' suites' }}
              </span>
            </div>

            <div class="suite-toolbar">
              <el-input
                v-model="suiteSearchQuery"
                clearable
                :placeholder="t('Search by suite, ID, owner, or description')"
              />
              <el-button type="primary" plain @click="openCreateSuiteDialog">
                {{ t('Create suite') }}
              </el-button>
            </div>

            <el-table
              :data="paginatedTestSuites"
              class="suite-table"
              border
              stripe
              :empty-text="t('No test suites match this search')"
            >
              <el-table-column
                v-for="column in testSuiteColumns"
                :key="column.field"
                :prop="column.renderer === 'actions' ? undefined : column.field"
                :label="column.label"
                :width="column.width"
                :min-width="column.minWidth"
                :align="column.align"
                :fixed="column.fixed"
                :show-overflow-tooltip="column.showOverflowTooltip"
              >
                <template #default="{ row }">
                  <el-tag
                    v-if="column.renderer === 'count'"
                    type="info"
                    effect="plain"
                  >
                    {{ tableValue(row, column) }}
                  </el-tag>
                  <div
                    v-else-if="column.renderer === 'actions'"
                    class="table-row-actions"
                  >
                    <el-button plain size="small" @click="editTestSuite(row)">
                      {{ t('Edit') }}
                    </el-button>
                    <el-button
                      type="success"
                      size="small"
                      @click="configureSuiteRun(row)"
                    >
                      {{ t('Run') }}
                    </el-button>
                  </div>
                  <template v-else>{{ tableValue(row, column) }}</template>
                </template>
              </el-table-column>
            </el-table>

            <div class="table-pagination">
              <el-pagination
                v-model:current-page="suiteCurrentPage"
                v-model:page-size="suitePageSize"
                :page-sizes="[10, 20, 50, 100]"
                :total="filteredTestSuites.length"
                layout="total, sizes, prev, pager, next"
              />
            </div>
          </section>
        </template>

        <template v-else-if="activeView === 'New Test Run'">
          <section v-if="testRunStarted" class="run-success">
            <span class="run-success-mark">✓</span>
            <p class="eyebrow">{{ t('Test run is active') }}</p>
            <h2>{{ newTestRun.name }}</h2>
            <p>
              {{ testRunExecution?.started?.length || selectedRunCaseCount }}
              test cases started in
              {{ t(['Execute only', 'Static inspection', 'Motion inspection'][newTestRun.inspectionMode]) }}
              mode on
              {{ selectedDevice?.name || selectedDevice?.model || newTestRun.device }}.
            </p>
            <div class="run-success-actions">
              <el-button type="primary" size="large" @click="selectView('Test Cases')">
                {{ t('Open test cases') }}
              </el-button>
              <el-button size="large" @click="testRunStarted = false">
                {{ t('Configure another run') }}
              </el-button>
            </div>
          </section>

          <section v-else class="run-layout">
            <article class="run-form-panel">
              <div class="run-page-intro">
                <p class="eyebrow">{{ t('Quality assurance workflow') }}</p>
                <h2>{{ t('Start a focused test run.') }}</h2>
                <p>{{ t('Name the run, choose a target device, and select the test cases for the tester.') }}</p>
              </div>

              <el-form class="run-form" label-position="top">
                <el-alert
                  v-if="testCasesError"
                  class="test-library-alert"
                  type="warning"
                  :title="t('Test case library unavailable')"
                  :description="testCasesError"
                  show-icon
                  :closable="false"
                />
                <el-alert
                  v-if="testRunError"
                  class="test-library-alert"
                  type="error"
                  :title="t('Test run failed')"
                  :description="testRunError"
                  show-icon
                  :closable="false"
                />
                <div class="run-form-grid">
                  <el-form-item :label="t('Test run name')" required>
                    <el-input
                      v-model="newTestRun.name"
                      :placeholder="t('Example: Release 2.4 regression')"
                    />
                  </el-form-item>
                  <el-form-item :label="t('Device')" required>
                    <el-select
                      v-model="newTestRun.device"
                      filterable
                      clearable
                      :loading="devicesLoading"
                      :disabled="Boolean(devicesError)"
                      :placeholder="t('Search by model, browser, or device ID')"
                      :no-match-text="t('No matching devices')"
                    >
                      <el-option
                        v-for="device in deviceOptions"
                        :key="device.id"
                        :label="(device.name || device.model || device.id) + ' (' + device.id + ')'"
                        :value="device.id"
                      />
                    </el-select>
                  </el-form-item>
                </div>

                <el-form-item :label="t('Test selection')" required>
                  <el-select
                    v-model="newTestRun.selections"
                    class="test-selection-select"
                    multiple
                    filterable
                    clearable
                    :placeholder="t('Search and select test cases')"
                    :no-match-text="t('No matching test cases')"
                  >
                    <el-option
                      v-for="option in testSelectionOptions"
                      :key="option.value"
                      :label="option.label"
                      :value="option.value"
                    />
                  </el-select>
                  <span class="selection-help">
                    {{ t('Choose one or more test cases for this run.') }}
                  </span>
                </el-form-item>

                <el-form-item :label="t('Inspection mode')" required>
                  <el-radio-group
                    v-model="newTestRun.inspectionMode"
                    class="inspection-mode"
                  >
                    <el-radio-button :value="0">
                      {{ t('Execute only') }}
                    </el-radio-button>
                    <el-radio-button :value="1">
                      {{ t('Static inspection') }}
                    </el-radio-button>
                    <el-radio-button :value="2">
                      {{ t('Motion inspection') }}
                    </el-radio-button>
                  </el-radio-group>
                </el-form-item>

                <el-form-item :label="t('Notes')">
                  <el-input
                    v-model="newTestRun.notes"
                    type="textarea"
                    :rows="4"
                    :placeholder="t('Add optional instructions or context')"
                  />
                </el-form-item>

                <div class="run-form-actions">
                  <el-button
                    type="primary"
                    size="large"
                    :loading="testRunStarting"
                    @click="startTestRun"
                  >
                    {{ t(testRunStarting ? 'Starting...' : 'Start test run') }}
                  </el-button>
                  <span>
                    {{ selectedRunCaseCount }} test cases ·
                    {{ t(['Execute only', 'Static inspection', 'Motion inspection'][newTestRun.inspectionMode]) }}
                  </span>
                </div>
              </el-form>
            </article>

            <aside class="run-summary-panel">
              <p class="eyebrow">{{ t('Run preview') }}</p>
              <h3>{{ newTestRun.name || t('Untitled test run') }}</h3>
              <div class="run-build">
                <span>{{ t('Target device') }}</span>
                <strong>
                  {{ selectedDevice
                    ? (selectedDevice.name || selectedDevice.model || selectedDevice.id)
                    : t('No device selected') }}
                </strong>
              </div>
              <div class="run-build">
                <span>{{ t('Inspection mode') }}</span>
                <strong>
                  {{ t(['Execute only', 'Static inspection', 'Motion inspection'][newTestRun.inspectionMode]) }}
                </strong>
              </div>
              <div class="run-selection">
                <span>{{ newTestRun.selectionType }}</span>
                <div v-if="selectedTestLabels.length" class="run-selection-tags">
                  <el-tag
                    v-for="label in selectedTestLabels"
                    :key="label"
                    type="info"
                    effect="light"
                  >
                    {{ label }}
                  </el-tag>
                </div>
                <strong v-else>
                  No {{ newTestRun.selectionType.toLowerCase() }} selected
                </strong>
                <small v-if="selectedRunCaseCount">
                  {{ selectedRunCaseCount }} unique test cases included
                </small>
              </div>

              <div class="run-readiness" :class="{ incomplete: !testRunReady }">
                <span class="readiness-dot"></span>
                <div>
                  <strong>{{ t(testRunReady ? 'Ready to start' : 'Setup incomplete') }}</strong>
                  <small>
                    {{ testRunReady
                      ? t('The test run can now be started.')
                      : t('Complete the three required fields.') }}
                  </small>
                </div>
              </div>
            </aside>
          </section>
        </template>

        <template v-else-if="activeView === 'Settings'">
          <section class="settings-page" v-loading="settingsLoading">
            <div class="settings-intro">
              <div>
                <p class="eyebrow">{{ t('Application settings') }}</p>
                <h2>{{ t('Settings') }}</h2>
                <p>{{ t('Edit the runtime parameters stored in the local config file.') }}</p>
              </div>
              <div class="settings-file">
                <span>{{ t('Config file') }}</span>
                <strong>app/config/settings.json</strong>
              </div>
            </div>

            <el-alert
              v-if="settingsError"
              class="settings-alert"
              type="error"
              :title="t('Settings unavailable')"
              :description="settingsError"
              show-icon
              :closable="false"
            />

            <el-form class="settings-form" label-position="top">
              <div class="settings-grid">
                <el-form-item :label="t('Project name')">
                  <el-input v-model="appSettings.projectName" />
                </el-form-item>
                <el-form-item :label="t('Release name')">
                  <el-input v-model="appSettings.releaseName" />
                </el-form-item>
                <el-form-item :label="t('Default environment')">
                  <el-input v-model="appSettings.defaultEnvironment" />
                </el-form-item>
                <el-form-item :label="t('Default owner')">
                  <el-input v-model="appSettings.defaultOwner" />
                </el-form-item>
                <el-form-item
                  class="settings-path-field"
                  :label="t('Test case library path')"
                >
                  <el-input
                    v-model="appSettings.testCaseLibraryPath"
                    placeholder="C:\\path\\to\\test-cases"
                  />
                  <span class="settings-field-help">
                    {{ t('Include all .py files in this directory and its subdirectories.') }}
                  </span>
                </el-form-item>
                <el-form-item
                  class="settings-path-field"
                  :label="t('Python executable path')"
                >
                  <el-input
                    v-model="appSettings.pythonExecutablePath"
                    placeholder="C:\\path\\to\\python.exe"
                  />
                  <span class="settings-field-help">
                    {{ t('Must point to a python.exe file.') }}
                  </span>
                </el-form-item>
                <el-form-item :label="t('Refresh interval')">
                  <el-input-number
                    v-model="appSettings.deviceRefreshSeconds"
                    :min="1"
                    :max="3600"
                    controls-position="right"
                  />
                  <span class="settings-unit">{{ t('seconds') }}</span>
                </el-form-item>
                <el-form-item :label="t('Table page size')">
                  <el-input-number
                    v-model="appSettings.tablePageSize"
                    :min="1"
                    :max="100"
                    controls-position="right"
                  />
                </el-form-item>
              </div>

              <div class="settings-toggle-row">
                <div>
                  <strong>{{ t('Auto-load devices') }}</strong>
                  <span>{{ t('Devices detected through HDC') }}</span>
                </div>
                <el-switch v-model="appSettings.autoLoadDevices" />
              </div>

              <div class="settings-actions">
                <el-button plain @click="loadSettings">
                  {{ t('Reload settings') }}
                </el-button>
                <el-button plain @click="resetSettings">
                  {{ t('Reset defaults') }}
                </el-button>
                <span class="settings-save-state">
                  {{ settingsSaving
                    ? t('Saving...')
                    : settingsSavedAt
                      ? t('Saved') + ' · ' + settingsSavedAt
                      : t('Saved') }}
                </span>
              </div>
            </el-form>
          </section>
        </template>

        <section v-else class="empty-view">
          <span class="empty-view-mark">{{ activeView.charAt(0) }}</span>
          <h2>{{ t(activeView) }}</h2>
          <p>{{ t('This area is ready for its next workflow.') }}</p>
          <el-button type="primary" @click="selectView('Test Cases')">
            {{ t('Open test cases') }}
          </el-button>
        </section>
      </main>

      <el-dialog
        v-model="createDialogVisible"
        :title="t(editingTestCaseId ? 'Edit test case' : 'Create test case')"
        width="min(520px, calc(100vw - 32px))"
      >
        <el-form label-position="top">
          <el-form-item :label="t('Test case title')" required>
            <el-input
              v-model="newTestCase.title"
              autofocus
              :placeholder="t('Describe the behavior to verify')"
              @keyup.enter="saveTestCase"
            />
          </el-form-item>
          <el-form-item :label="t('Owner')">
            <el-input v-model="newTestCase.owner" :placeholder="t('Name or team')" />
          </el-form-item>
        </el-form>
        <template #footer>
          <el-button @click="createDialogVisible = false">{{ t('Cancel') }}</el-button>
          <el-button type="primary" @click="saveTestCase">
            {{ t(editingTestCaseId ? 'Save changes' : 'Create test case') }}
          </el-button>
        </template>
      </el-dialog>

      <el-dialog
        v-model="createSuiteDialogVisible"
        :title="t(editingTestSuiteId ? 'Edit test suite' : 'Create test suite')"
        width="min(680px, calc(100vw - 32px))"
      >
        <el-form label-position="top">
          <el-form-item :label="t('Suite name')" required>
            <el-input
              v-model="newTestSuite.name"
              autofocus
              :placeholder="t('Example: Release 2.5 critical path')"
            />
          </el-form-item>
          <el-form-item :label="t('Description')">
            <el-input
              v-model="newTestSuite.description"
              type="textarea"
              :rows="3"
              :placeholder="t('Explain when this suite should be used')"
            />
          </el-form-item>
          <el-form-item :label="t('Owner')">
            <el-input
              v-model="newTestSuite.owner"
              :placeholder="t('Name or team')"
            />
          </el-form-item>
          <el-form-item :label="t('Test cases')" required>
            <el-select
              v-model="newTestSuite.caseIds"
              class="suite-case-select"
              multiple
              filterable
              clearable
              collapse-tags
              collapse-tags-tooltip
              :placeholder="t('Search and select test cases')"
            >
              <el-option
                v-for="testCase in testCases"
                :key="testCase.id"
                :label="testCase.id + ' - ' + testCase.title"
                :value="testCase.id"
              />
            </el-select>
            <span class="selection-help">
              {{ isChinese
                ? '已选择 ' + newTestSuite.caseIds.length + ' 个测试用例。'
                : newTestSuite.caseIds.length + ' test cases selected.' }}
            </span>
          </el-form-item>
        </el-form>
        <template #footer>
          <el-button @click="createSuiteDialogVisible = false">{{ t('Cancel') }}</el-button>
          <el-button type="primary" @click="saveTestSuite">
            {{ t(editingTestSuiteId ? 'Save changes' : 'Create test suite') }}
          </el-button>
        </template>
      </el-dialog>
    </div>
  `,
};

createApp(App).use(ElementPlus).mount("#app");
