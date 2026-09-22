<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { BadgeCheck, Check, Plus, RefreshCw, X } from 'lucide-vue-next'
import { ElMessage } from 'element-plus'
import { mitigationApi, profileApi, routeApi } from '@/api/domain'
import { useAuth } from '@/hooks/useAuth'
import type { AllergenProfile, ProcessRoute } from '@/types/domain'
import { measureTypeLabels, mitigationLabels, type MeasureType, type MitigationMeasure, type MitigationStatus } from '@/types/mitigation'
import { dateTime } from '@/utils/format'

const auth = useAuth()
const measures = ref<MitigationMeasure[]>([])
const routes = ref<ProcessRoute[]>([])
const profiles = ref<AllergenProfile[]>([])
const loading = ref(false)
const workingId = ref<number>()
const routeFilter = ref<number>()
const statusFilter = ref<MitigationStatus>()
const createOpen = ref(false)
const saving = ref(false)
const reviewOpen = ref(false)
const reviewDecision = ref<'approved' | 'rejected'>('approved')
const reviewReason = ref('')
const selected = ref<MitigationMeasure>()
const form = reactive({ route_id: undefined as number | undefined, allergen: '', source_step_code: '', target_step_code: '', measure_type: 'cleaning' as MeasureType, completed_at: '', evidence_note: '' })

const routesById = computed(() => Object.fromEntries(routes.value.map((route) => [route.id, route])))
const activeRoutes = computed(() => routes.value.filter((route) => route.route_status === 'active'))
const formRoute = computed(() => routes.value.find((route) => route.id === form.route_id))
const formAllergens = computed(() => {
  if (!formRoute.value) return []
  const ids = new Set(formRoute.value.ordered_steps_json.map((step) => step.profile_id))
  const found = new Set<string>()
  profiles.value.filter((profile) => ids.has(profile.id)).forEach((profile) => profile.allergens_json.forEach((item) => found.add(item)))
  return [...found].sort()
})
const countOf = (status: MitigationStatus) => measures.value.filter((item) => item.mitigation_status === status).length
const statusLabel = (status: MitigationStatus) => mitigationLabels[status]
const measureLabel = (type: MeasureType) => measureTypeLabels[type]
const today = () => new Date().toISOString().slice(0, 10)

async function load() {
  loading.value = true
  try { measures.value = (await mitigationApi.list({ route_id: routeFilter.value, status: statusFilter.value, page_size: 100 })).items }
  finally { loading.value = false }
}
async function loadInputs() {
  routes.value = (await routeApi.list({ page_size: 100 })).items
  profiles.value = (await profileApi.list({ page_size: 100 })).items
}
function openCreate() {
  const route = activeRoutes.value[0]
  Object.assign(form, { route_id: route?.id, allergen: '', source_step_code: '', target_step_code: '', measure_type: 'cleaning', completed_at: today(), evidence_note: '' })
  createOpen.value = true
}
async function create() {
  if (!form.route_id || !form.allergen || !form.source_step_code || !form.target_step_code || !form.completed_at || form.evidence_note.trim().length < 4) return
  saving.value = true
  try {
    const measure = await mitigationApi.create({ route_id: form.route_id, allergen: form.allergen, source_step_code: form.source_step_code, target_step_code: form.target_step_code, measure_type: form.measure_type, completed_at: form.completed_at, evidence_note: form.evidence_note })
    ElMessage.success(`缓解措施 #${measure.id} 已登记，等待复核`)
    createOpen.value = false
    await load()
  } finally { saving.value = false }
}
function openReview(measure: MitigationMeasure, decision: 'approved' | 'rejected') { selected.value = measure; reviewDecision.value = decision; reviewReason.value = ''; reviewOpen.value = true }
async function review() {
  if (!selected.value || reviewReason.value.trim().length < 4) return
  workingId.value = selected.value.id
  try {
    await mitigationApi.review(selected.value.id, reviewDecision.value, reviewReason.value)
    ElMessage.success(reviewDecision.value === 'approved' ? '措施已通过，矩阵将显示该路径已缓解' : '措施已拒绝')
    reviewOpen.value = false
    await load()
  } finally { workingId.value = undefined }
}
onMounted(async () => { await Promise.all([loadInputs(), load()]) })
</script>

<template>
  <header class="page-header"><div class="page-header-copy"><h1>缓解复核</h1><p>高风险路径的清洗与换线措施、证据与失效追踪</p></div><div class="page-header-actions"><el-button v-if="auth.canEdit.value" type="primary" :icon="Plus" :disabled="!activeRoutes.length" @click="openCreate">登记措施</el-button></div></header>
  <section class="content-band stack">
    <div class="metric-strip"><div class="metric"><span>措施总数</span><strong>{{ measures.length }}</strong></div><div class="metric"><span>待复核</span><strong>{{ countOf('pending_review') }}</strong></div><div class="metric"><span>已通过</span><strong>{{ countOf('approved') }}</strong></div><div class="metric"><span>已失效·待处理</span><strong>{{ countOf('invalidated') }}</strong></div></div>
    <div class="toolbar"><el-select v-model="routeFilter" clearable filterable placeholder="全部路线" style="width: 280px" @change="load"><el-option v-for="route in routes" :key="route.id" :label="`${route.route_code} · ${route.product_name}`" :value="route.id" /></el-select><el-select v-model="statusFilter" clearable placeholder="全部状态" style="width: 180px" @change="load"><el-option v-for="(label, status) in mitigationLabels" :key="status" :label="label" :value="status" /></el-select><el-tooltip content="刷新"><el-button class="toolbar-spacer" :icon="RefreshCw" text circle aria-label="刷新" @click="load" /></el-tooltip></div>
    <div class="data-surface">
      <div class="surface-header"><BadgeCheck :size="18" /><h2>缓解措施</h2><span>同一路径仅一条待复核</span></div>
      <el-table v-loading="loading" :data="measures" row-key="id">
        <el-table-column label="编号" width="76"><template #default="{ row }"><span class="mono">#{{ row.id }}</span></template></el-table-column>
        <el-table-column label="路线 / 版本" min-width="170"><template #default="{ row }"><strong>{{ routesById[row.route_id]?.route_code || `路线 #${row.route_id}` }}</strong><div class="muted row-sub">登记于 v{{ row.route_version }}</div></template></el-table-column>
        <el-table-column prop="allergen" label="过敏原" width="105" />
        <el-table-column label="路径" min-width="150"><template #default="{ row }"><span class="mono">{{ row.source_step_code }} → {{ row.target_step_code }}</span></template></el-table-column>
        <el-table-column label="措施" width="80"><template #default="{ row }">{{ measureLabel(row.measure_type) }}</template></el-table-column>
        <el-table-column label="完成日期" width="115"><template #default="{ row }">{{ row.completed_at.slice(0, 10) }}</template></el-table-column>
        <el-table-column prop="evidence_note" label="证据" min-width="220" show-overflow-tooltip />
        <el-table-column label="状态" width="140"><template #default="{ row }"><span class="status-pill" :class="row.mitigation_status">{{ statusLabel(row.mitigation_status) }}</span></template></el-table-column>
        <el-table-column label="复核" min-width="150"><template #default="{ row }"><div v-if="row.reviewed_at" class="review-cell"><span>{{ dateTime(row.reviewed_at) }}</span><small>{{ row.review_reason }}</small></div><span v-else-if="row.invalidated_at" class="muted">失效于 {{ dateTime(row.invalidated_at) }}</span><span v-else class="muted">—</span></template></el-table-column>
        <el-table-column v-if="auth.canReview.value" label="操作" width="150" fixed="right"><template #default="{ row }"><template v-if="row.mitigation_status === 'pending_review'"><el-button type="success" text :icon="Check" :loading="workingId === row.id" @click="openReview(row, 'approved')">通过</el-button><el-button type="danger" text :icon="X" @click="openReview(row, 'rejected')">拒绝</el-button></template></template></el-table-column>
        <template #empty><div class="empty-state"><div><strong>暂无缓解措施</strong><span>在矩阵页选择高风险路径或在此登记清洗 / 换线措施</span></div></div></template>
      </el-table>
    </div>
  </section>

  <el-dialog v-model="createOpen" title="登记缓解措施" width="620px" destroy-on-close>
    <el-form label-position="top">
      <el-form-item label="active 工艺路线" required><el-select v-model="form.route_id" filterable style="width: 100%" @change="form.allergen = ''; form.source_step_code = ''; form.target_step_code = ''"><el-option v-for="route in activeRoutes" :key="route.id" :label="`${route.route_code} · ${route.product_name} · v${route.version}`" :value="route.id" /></el-select></el-form-item>
      <div class="mitigation-grid"><el-form-item label="来源步骤" required><el-select v-model="form.source_step_code" :disabled="!formRoute" style="width: 100%"><el-option v-for="step in formRoute?.ordered_steps_json" :key="step.step_code" :label="`${step.step_code} · ${step.step_name}`" :value="step.step_code" /></el-select></el-form-item><el-form-item label="目标步骤" required><el-select v-model="form.target_step_code" :disabled="!formRoute" style="width: 100%"><el-option v-for="step in formRoute?.ordered_steps_json.filter((step) => step.step_code !== form.source_step_code)" :key="step.step_code" :label="`${step.step_code} · ${step.step_name}`" :value="step.step_code" /></el-select></el-form-item></div>
      <div class="mitigation-grid"><el-form-item label="过敏原" required><el-select v-model="form.allergen" :disabled="!formAllergens.length" style="width: 100%"><el-option v-for="item in formAllergens" :key="item" :label="item" :value="item" /></el-select></el-form-item><el-form-item label="措施类型" required><el-select v-model="form.measure_type" style="width: 100%"><el-option label="清洗" value="cleaning" /><el-option label="换线" value="line_change" /></el-select></el-form-item></div>
      <el-form-item label="完成日期（不能晚于提交日）" required><el-date-picker v-model="form.completed_at" type="date" value-format="YYYY-MM-DD" :disabled-date="(date: Date) => date.getTime() > Date.now()" style="width: 100%" /></el-form-item>
      <el-form-item label="证据记录" required><el-input v-model="form.evidence_note" type="textarea" :rows="3" maxlength="1000" show-word-limit placeholder="清洗验证、换线检查表或检测记录" /></el-form-item>
    </el-form>
    <template #footer><el-button @click="createOpen = false">取消</el-button><el-button type="primary" :loading="saving" :disabled="!form.route_id || !form.allergen || !form.source_step_code || !form.target_step_code || !form.completed_at || form.evidence_note.trim().length < 4" @click="create">提交复核</el-button></template>
  </el-dialog>

  <el-dialog v-model="reviewOpen" :title="reviewDecision === 'approved' ? '通过缓解措施' : '拒绝缓解措施'" width="540px">
    <div v-if="selected" class="review-target">措施 #{{ selected.id }} · {{ selected.allergen }} · {{ selected.source_step_code }} → {{ selected.target_step_code }} · {{ measureTypeLabels[selected.measure_type] }}</div>
    <el-form label-position="top"><el-form-item label="复核理由" required><el-input v-model="reviewReason" type="textarea" :rows="4" maxlength="1000" show-word-limit placeholder="记录证据判断与后续处置依据" /></el-form-item></el-form>
    <template #footer><el-button @click="reviewOpen = false">取消</el-button><el-button :type="reviewDecision === 'approved' ? 'success' : 'danger'" :loading="workingId === selected?.id" :disabled="reviewReason.trim().length < 4" @click="review">确认{{ reviewDecision === 'approved' ? '通过' : '拒绝' }}</el-button></template>
  </el-dialog>
</template>

<style scoped>
.row-sub { margin-top: 3px; font-size: 11px; }.mitigation-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 14px; }.review-cell span, .review-cell small { display: block; }.review-cell small { margin-top: 2px; color: var(--muted); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }.review-target { margin-bottom: 16px; padding: 10px 12px; background: var(--surface-muted); border: 1px solid var(--line); font: 12px "SFMono-Regular", Consolas, monospace; }
@media (max-width: 640px) { .mitigation-grid { grid-template-columns: 1fr; gap: 0; } }
</style>
