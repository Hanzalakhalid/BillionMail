import { createRouter, createWebHistory, RouteRecordRaw } from 'vue-router'
import { is, isDev } from '@/utils'

// Routes reflect list
const routesReflectList = [
	'Overview',
	'Email Marketing',
	'template',
	'Send API',
	'Contacts',
	'Sequences',
	'Leads',
	'Enrichment',
	'MailDomain',
	'MailBoxes',
	'SMTP',
	'Logs',
	'Settings',
	'Automation',
	'Video Outreach',
	'Ad Finder',
	'SMS',
	'WhatsApp',
	'Push',
	'Team Members',
]

// Import routes from modules
const modules = import.meta.webpackContext('./modules', {
	// Whether to search for subdirectories
	recursive: false,
	// Keys look like "./overview.ts"; skip "./overview.test.ts"
	regExp: /^\.\/[^.]+\.ts$/,
})

// Module routes
export let menuList: RouteRecordRaw[] = []

// Iterate through the module list to generate module routes
for (const path of modules.keys()) {
	const mod = modules(path)
	if (is<{ default: RouteRecordRaw }>(mod, 'Module')) {
		menuList.push(mod.default)
	}
}

// Sort module routes
// Unlisted routes go last; sorting (not index assignment) avoids holes for listed titles without a module
const routeOrder = (route: RouteRecordRaw) => {
	const index = routesReflectList.indexOf(route.meta!.title as string)
	return index === -1 ? routesReflectList.length : index
}
menuList = menuList.sort((a, b) => routeOrder(a) - routeOrder(b))

const otherArray: RouteRecordRaw[] = []

if (isDev) {
	otherArray.push({
		path: '/test',
		name: 'Test',
		component: () => import('@/views/test/index.vue'),
	})
}

export const routes: RouteRecordRaw[] = [
	{
		path: '/login',
		name: 'Login',
		component: () => import('@/views/login/index.vue'),
	},
	{ path: '/register', name: 'Register', component: () => import('@/views/auth/register.vue') },
	{ path: '/forgot-password', name: 'ForgotPassword', component: () => import('@/views/auth/forgot-password.vue') },
	{ path: '/reset-password', name: 'ResetPassword', component: () => import('@/views/auth/reset-password.vue') },
	{ path: '/verify-email', name: 'VerifyEmail', component: () => import('@/views/auth/verify-email.vue') },
	{ path: '/accept-invitation', name: 'AcceptInvitation', component: () => import('@/views/auth/accept-invitation.vue') },
	{
		path: '/',
		redirect: '/overview',
	},
	...menuList,
	...otherArray,
]

const router = createRouter({
	history: createWebHistory('/'),
	routes,
	strict: false,
	scrollBehavior: () => ({ left: 0, top: 0 }),
})

export default router
