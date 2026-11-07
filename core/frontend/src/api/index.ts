import axios, { AxiosRequestConfig } from 'axios'
import router from '@/router'
import { useUserStore } from '@/store'
import { apiUrlPrefix, isObject, Message } from '@/utils'

interface FetchOptions {
	prefix: string
	loading: string
	loadFn: () => void
	cancelResInterceptor: boolean
	rawResponse?: boolean
}

const instance = axios.create({
	baseURL: apiUrlPrefix,
	timeout: 600000,
	headers: {
		'Content-Type': 'application/json',
	},
	fetchOptions: {
		prefix: '/api',
		successMessage: false,
	},
})

const whitePathList = [
	'/login', '/refresh-token', '/v1/auth/register', '/v1/auth/verify-email',
	'/v1/auth/forgot-password', '/v1/auth/reset-password',
]

let refreshPromise: Promise<string> | null = null

const refreshAccessToken = async (): Promise<string> => {
	if (refreshPromise) return refreshPromise
	const userStore = useUserStore()
	refreshPromise = axios
		.post(`${apiUrlPrefix}/api/refresh-token`, { refreshToken: userStore.login.refresh_token })
		.then(response => {
			const payload = response.data?.data
			if (!response.data?.success || !payload?.token || !payload?.refreshToken) {
				throw new Error('Refresh token rejected')
			}
			userStore.setLoginInfo({
				token: payload.token,
				refresh_token: payload.refreshToken,
				ttl: payload.ttl,
			})
			return payload.token as string
		})
		.finally(() => { refreshPromise = null })
	return refreshPromise
}

// 存储所有请求的 AbortController
const controllerStore = new Map<string, AbortController>()

// 生成唯一请求标识
const generateReqKey = (config: AxiosRequestConfig): string => {
	const { method, url, params, data } = config
	return `${method}-${url}-${JSON.stringify(params)}-${JSON.stringify(data)}`
}

// 添加请求到控制器存储
const addController = (config: AxiosRequestConfig): void => {
	const key = generateReqKey(config)

	// 如果已有相同请求则取消前一个
	if (controllerStore.has(key)) {
		controllerStore.get(key)?.abort()
		controllerStore.delete(key)
	}

	// 创建新控制器
	const controller = new AbortController()
	config.signal = controller.signal
	controllerStore.set(key, controller)
}

// 从存储中移除控制器
const removeController = (config: AxiosRequestConfig): void => {
	const key = generateReqKey(config)
	if (controllerStore.has(key)) {
		controllerStore.delete(key)
	}
}

// 请求拦截器 增加token
instance.interceptors.request.use(config => {
	addController(config)
	const { headers, url } = config
	if (!whitePathList.includes(url || '')) {
		const userStore = useUserStore()
		headers.Authorization = `Bearer ${userStore.login.token}`
	}
	return config
})

// 请求拦截器 处理请求前缀
instance.interceptors.request.use(config => {
	const { fetchOptions } = config
	if (isObject<FetchOptions>(fetchOptions)) {
		const prefix = fetchOptions.prefix || ''
		const url = config.url || ''
		// Axios reuses the already-prefixed config after a token refresh.
		if (prefix && url !== prefix && !url.startsWith(`${prefix}/`) && !url.startsWith(`${prefix}?`)) {
			config.url = prefix + url
		}
	}
	return config
})

// 请求拦截器 处理请求前缀
instance.interceptors.request.use(config => {
	const { fetchOptions } = config
	if (isObject<FetchOptions>(fetchOptions) && fetchOptions.loading) {
		const { close } = Message.loading(fetchOptions.loading)
		fetchOptions.loadFn = close
	}
	return config
})

// 成功、失败处理
instance.interceptors.response.use(
	async response => {
		removeController(response.config)

		const fetchOptions = response.config.fetchOptions as FetchOptions | undefined
		const { code, data, msg, success } = response.data || {}

		if (fetchOptions?.cancelResInterceptor) {
			return Promise.resolve(data)
		}

		if (fetchOptions?.loadFn) {
			fetchOptions.loadFn()
		}

		// New versioned resources use direct JSON responses. Existing endpoints
		// retain their envelope, including application errors returned with 200.
		if (fetchOptions?.rawResponse && !(success === false && typeof code === 'number')) {
			return Promise.resolve(response.data)
		}

		if (response.data.type === 'application/octet-stream') {
			const url = window.URL.createObjectURL(new Blob([response.data]))
			const link = document.createElement('a')
			link.href = url
			const disposition = response.headers['content-disposition']
			const filename = disposition
				? decodeURIComponent(disposition.split('filename=')[1])
				: 'downloaded_file'
			link.setAttribute('download', filename)
			document.body.appendChild(link)
			link.click()
			document.body.removeChild(link)
			window.URL.revokeObjectURL(url)
			return Promise.resolve(data)
		}

		if (code === 0 && success) {
			if (fetchOptions?.successMessage) {
				Message.success(msg)
			}
			return Promise.resolve(data)
		}
		if (!success && msg) {
			Message.error(msg, {
				close: true,
			})
		}
		if (code === 401) {
			const userStore = useUserStore()
			const retryConfig = response.config as AxiosRequestConfig & { _retry?: boolean }
			if (!retryConfig._retry && userStore.login.refresh_token && !retryConfig.url?.includes('/refresh-token')) {
				retryConfig._retry = true
				try {
					const token = await refreshAccessToken()
					retryConfig.headers = { ...retryConfig.headers, Authorization: `Bearer ${token}` }
					return instance.request(retryConfig)
				} catch {
					userStore.resetLoginInfo()
					router.push('/login')
				}
			} else {
				userStore.resetLoginInfo()
				router.push('/login')
			}
		}
		return Promise.reject(response.data)
	},
	error => {
		if (!axios.isCancel(error)) {
			removeController(error.config || {})
		}
		return Promise.reject(error)
	}
)

// 取消所有进行中的请求
const clearPendingRequests = (): void => {
	controllerStore.forEach((controller, key) => {
		controller.abort()
		controllerStore.delete(key)
	})
}

export { instance, clearPendingRequests }
