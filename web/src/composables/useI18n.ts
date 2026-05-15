import { ref, computed } from 'vue'

type Lang = 'zh' | 'en'

const messages: Record<Lang, Record<string, string>> = {
  zh: {
    app_title: 'IEC104 模拟器管理系统',
    nav_config: '配置管理',
    nav_monitor: '运行监控',
    nav_trend: '实时趋势',
    trend_desc: '选取多个实例的测点，放在同一张图上对比趋势。每 5 秒轮询一次，最长保留 1 小时数据。',
    trend_add: '+ 添加测点',
    trend_selected: '已选测点',
    trend_lines: '条线',
    trend_empty: '点击上方「+ 添加测点」选择要跟踪的数据',
    trend_chart: '实时趋势',
    trend_point: '点',
    trend_last: '上次',
    trend_add_title: '添加趋势测点',
    trend_instance: '实例',
    trend_point_label: '测点',
    trend_alias: '别名',
    trend_cancel: '取消',
    trend_confirm: '确认添加',
    trend_no_data: '请先添加测点开始监控。',
    theme_dark: '深色',
    theme_light: '浅色',
    lang_zh: '中文',
    lang_en: 'English',
    user_login: '登录',
    user_logout: '退出',
    user_settings: '用户设置',
  },
  en: {
    app_title: 'IEC104 Simulator',
    nav_config: 'Config',
    nav_monitor: 'Monitor',
    nav_trend: 'Trend',
    trend_desc: 'Select points from multiple instances to compare on one chart. Polls every 5s, retains up to 1 hour.',
    trend_add: '+ Add Point',
    trend_selected: 'Selected Points',
    trend_lines: 'lines',
    trend_empty: 'Click "+ Add Point" above to start tracking',
    trend_chart: 'Real-time Trend',
    trend_point: 'pts',
    trend_last: 'updated',
    trend_add_title: 'Add Trend Point',
    trend_instance: 'Instance',
    trend_point_label: 'Point',
    trend_alias: 'Alias',
    trend_cancel: 'Cancel',
    trend_confirm: 'Confirm',
    trend_no_data: 'Add points to start monitoring.',
    theme_dark: 'Dark',
    theme_light: 'Light',
    lang_zh: '中文',
    lang_en: 'English',
    user_login: 'Login',
    user_logout: 'Logout',
    user_settings: 'Settings',
  },
}

const lang = ref<Lang>(
  (localStorage.getItem('lang') as Lang) || 'zh'
)

export function useI18n() {
  function t(key: string): string {
    return messages[lang.value][key] || key
  }

  function setLang(l: Lang) {
    lang.value = l
    localStorage.setItem('lang', l)
  }

  return { lang, t, setLang }
}
