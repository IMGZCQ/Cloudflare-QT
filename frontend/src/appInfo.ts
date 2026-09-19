// 应用元信息：编译时通过 VITE_APP_VERSION 注入，未注入时回退到开发版本
export const appVersion = import.meta.env.VITE_APP_VERSION || 'dev'
