<template>
	<n-select
		v-if="companyStore.companies.length"
		class="company-switcher"
		:value="companyStore.activeCompanyId"
		:options="options"
		:loading="companyStore.loading"
		:consistent-menu-width="false"
		aria-label="Active company workspace"
		@update:value="companyStore.switchTo" />
</template>

<script lang="ts" setup>
import { useCompanyStore } from '@/store'

const companyStore = useCompanyStore()
const options = computed(() =>
	companyStore.companies.map(company => ({
		label: `${company.name} · ${company.role}`,
		value: company.id,
	}))
)

onMounted(() => companyStore.load())
</script>

<style scoped>
.company-switcher { width: min(260px, 24vw); }
</style>
