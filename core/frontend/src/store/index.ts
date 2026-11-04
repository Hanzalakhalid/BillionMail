import { createPinia } from 'pinia'
import piniaPluginPersistedstate from 'pinia-plugin-persistedstate'
import useMenuStore from './modules/menu'
import useUserStore from './modules/user'
import useGlobalStore from './modules/global'
import useThemeStore from './modules/theme'
import useInstanceStore from './modules/instance'
import useCompanyStore from './modules/company'

const pinia = createPinia()
pinia.use(piniaPluginPersistedstate)

export { useMenuStore, useUserStore, useGlobalStore, useThemeStore, useInstanceStore, useCompanyStore }

export default pinia
