import { defineStore } from 'pinia'
import { listCompanies, switchCompany, type Company } from '@/api/modules/companies'
import { isObject } from '@/utils'
import useUserStore from './user'

interface CompanyListResponse {
	companies: Company[]
	active_company_id: string
}

interface CompanySwitchResponse {
	token: string
	ttl: number
	active_company_id: string
	role: Company['role']
}

export default defineStore('CompanyStore', () => {
	const companies = ref<Company[]>([])
	const activeCompanyId = ref('')
	const loading = ref(false)

	const activeCompany = computed(() => companies.value.find(item => item.id === activeCompanyId.value))

	const load = async () => {
		loading.value = true
		try {
			const res = await listCompanies()
			if (isObject<CompanyListResponse>(res)) {
				companies.value = res.companies ?? []
				activeCompanyId.value = res.active_company_id
			}
		} finally {
			loading.value = false
		}
	}

	const switchTo = async (companyId: string) => {
		if (!companyId || companyId === activeCompanyId.value) return
		const res = await switchCompany(companyId)
		if (!isObject<CompanySwitchResponse>(res)) return
		const userStore = useUserStore()
		userStore.replaceAccessToken(res.token, res.ttl)
		activeCompanyId.value = res.active_company_id
		window.location.reload()
	}

	const reset = () => {
		companies.value = []
		activeCompanyId.value = ''
	}

	return { companies, activeCompanyId, activeCompany, loading, load, switchTo, reset }
})
