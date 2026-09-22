<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { AlertTriangle, Calculator, RefreshCw, Sparkles } from 'lucide-vue-next'
import { ElMessage } from 'element-plus'
import { assessmentApi, mitigationApi, routeApi } from '@/api/domain'
import EvidencePathPanel from '@/components/common/EvidencePathPanel.vue'
import RiskBadge from '@/components/common/RiskBadge.vue'
import { useAuth } from '@/hooks/useAuth'
import type { MatrixResult, ProcessRoute } from '@/types/domain'
import type { MatrixCell, RiskItem } from '@/types/assessment'
import { measureTypeLabels, type MeasureType } from '@/types/mitigation'
import { percent } from '@/utils/format'

const auth = useAuth()
const routes = ref<ProcessRoute[]>([])
const routeId = ref<number>()
const result = ref<MatrixResult>()
const loading = ref(false)
const error = ref('')
const allergen = ref('')
const selectedRisk = ref<RiskItem>()
const mitigateOpen = ref(false)
const saving = ref(false)
const mitigateForm = reactive({ item: undefined as RiskItem | undefined, measure_type: 'cleaning' as MeasureType, completed_at: '', evidence_note: '' })
const filteredCells = computed(() => result.value?.matrix.filter((item) => !allergen.value || item.allergen === allergen.value) || [])
const filteredRisks = computed(() => result.value?.risk_items.filter((item) => !allergen.value || item.allergen === allergen.value) || [])

async function loadRoutes() { routes.value = (await routeApi.list({ status: 'active', page_size: 100 })).items; if (!routeId.value && routes.value.length) routeId.value = routes.value[0].id }
async function compute() {
  if (!routeId.value) return
  loading.value = true; error.value = ''; selectedRisk.value = undefined
  try { result.value = await assessmentApi.matrix(routeId.value); selectedRisk.value = result.value.risk_items[0] }
  catch { result.value = undefined; error.value = '矩阵计算未完成，请检查路线步骤和接触边。' }
  finally { loading.value = false }
}
function selectCell(cell: MatrixCell) { selectedRisk.value = filteredRisks.value.find((item) => item.target_step_code === cell.target_step_code && item.allergen === cell.allergen) }
function openMitigate(item: RiskItem) { mitigateForm.item = item; mitigateForm.measure_type = 'cleaning'; mitigateForm.completed_at = new Date().toISOString().slice(0, 10); mitigateForm.evidence_note = ''; mitigateOpen.value = true }
async function submitMitigation() {
  const item = mitigateForm.item
  if (!item || !routeId.value || !mitigateForm.completed_at || mitigateForm.evidence_note.trim().length < 4) return
  saving.value = true
  try {
    const measure = await mitigationApi.create({ route_id: routeId.value, allergen: item.allergen, source_step_code: item.source_step_code, target_step_code: item.target_step_code, measure_type: mitigateForm.measure_type, completed_at: mitigateForm.completed_at, evidence_note: mitigateForm.evidence_note })
    ElMessage.success(`缓解措施 #${measure.id} 已登记，复核通过后矩阵将标记已缓解`)
    mitigateOpen.value = false
  } finally { saving.value = false }
}
const cellKey = (cell: MatrixCell) => `${cell.target_step_code}:${cell.allergen}`
const riskKey = (item: RiskItem) => `${item.source_profile_id}:${item.allergen}:${item.path.join('>')}`
const measureLabel = (type: MeasureType) => measureTypeLabels[type]
onMounted(async () => { await loadRoutes(); await compute() })
</script>

<template>
  <header class="page-header"><div class="page-header-copy"><h1>交叉接触矩阵</h1><p>带权传播、声明分离与清洗证据</p></div><div class="page-header-actions"><el-button type="primary" :icon="Calculator" :loading="loading" :disabled="!routeId" @click="compute">计算矩阵</el-button></div></header>
  <section class="content-band stack">
    <div class="toolbar"><el-select v-model="routeId" filterable placeholder="选择 active 路线" style="width: 300px" @change="compute"><el-option v-for="route in routes" :key="route.id" :label="`${route.route_code} · ${route.product_name} · v${route.version}`" :value="route.id" /></el-select><el-select v-model="allergen" clearable placeholder="全部过敏原" style="width: 170px"><el-option v-for="item in result?.propagated_allergens" :key="item" :label="item" :value="item" /></el-select><el-tooltip content="重新计算"><el-button :icon="RefreshCw" circle aria-label="重新计算" :disabled="!routeId" @click="compute" /></el-tooltip><span v-if="result" class="toolbar-spacer subtle-count">阈值 {{ result.thresholds.version }} · 最大深度 {{ result.max_depth }}</span></div>

    <div v-if="result" class="metric-strip"><div class="metric"><span>传播过敏原</span><strong>{{ result.propagated_allergens.length }}</strong></div><div class="metric"><span>证据路径</span><strong>{{ result.risk_items.length }}</strong></div><div class="metric"><span>已缓解路径</span><strong>{{ result.risk_items.filter((item) => item.mitigation).length }}</strong></div><div class="metric"><span>最高风险</span><RiskBadge :level="result.highest_risk_level" /></div></div>
    <div v-if="error" class="matrix-error"><AlertTriangle :size="19" /><span>{{ error }}</span></div>

    <div class="section-grid">
      <div class="stack">
        <div class="data-surface">
          <div class="surface-header"><h2>目标步骤 × 过敏原</h2><span>{{ filteredCells.length }} 个矩阵单元</span></div>
          <div v-if="loading" class="loading-state">正在构建接触图并传播路径…</div>
          <el-table v-else-if="result" :data="filteredCells" :row-key="cellKey" highlight-current-row @row-click="selectCell">
            <el-table-column label="目标步骤" min-width="155"><template #default="{ row }"><strong class="mono">{{ row.target_step_code }}</strong><div class="muted cell-sub">{{ row.target_step_name }}</div></template></el-table-column>
            <el-table-column prop="allergen" label="过敏原" min-width="110" />
            <el-table-column label="最大分数" width="120"><template #default="{ row }"><span class="score">{{ percent(row.max_raw_score) }}</span></template></el-table-column>
            <el-table-column label="风险" width="125"><template #default="{ row }"><RiskBadge :level="row.risk_level" /></template></el-table-column>
            <el-table-column prop="path_count" label="路径数" width="78" />
            <el-table-column label="缓解" width="105"><template #default="{ row }"><span v-if="row.mitigated_paths === row.path_count && row.path_count > 0" class="status-pill approved">已缓解</span><span v-else-if="row.mitigated_paths > 0" class="status-pill calculating">{{ row.mitigated_paths }}/{{ row.path_count }} 缓解</span><span v-else class="muted">—</span></template></el-table-column>
            <el-table-column label="分类" width="90"><template #default="{ row }"><span class="status-pill" :class="row.declared ? 'accepted' : 'pending_review'">{{ row.declared ? '已声明' : '传播' }}</span></template></el-table-column>
            <template #empty><div class="empty-state"><div><strong>没有可传播路径</strong><span>检查已启用接触边及其衰减参数</span></div></div></template>
          </el-table>
          <div v-else-if="!loading && !error" class="empty-state"><div><strong>请选择路线</strong><span>计算后显示真实矩阵单元</span></div></div>
        </div>

        <div v-if="result" class="data-surface">
          <div class="surface-header"><h2>传播路径</h2><span>按累计分数降序</span></div>
          <el-table :data="filteredRisks" :row-key="riskKey" size="small" @row-click="(row: RiskItem) => selectedRisk = row">
            <el-table-column label="来源" min-width="150"><template #default="{ row }"><span class="mono">{{ row.source_profile_code }}</span><div class="muted cell-sub">{{ row.source_material }}</div></template></el-table-column>
            <el-table-column label="路径" min-width="220"><template #default="{ row }"><span class="mono">{{ row.path.join(' → ') }}</span></template></el-table-column>
            <el-table-column prop="allergen" label="过敏原" width="105" />
            <el-table-column label="累计风险" width="145"><template #default="{ row }"><RiskBadge :level="row.risk_level" :score="row.raw_score" /></template></el-table-column>
            <el-table-column label="缓解" width="120"><template #default="{ row }"><el-tooltip v-if="row.mitigation" :content="`${measureLabel(row.mitigation.measure_type)} · 完成于 ${row.mitigation.completed_at}`"><span class="status-pill approved">已缓解</span></el-tooltip><el-button v-else-if="auth.canEdit.value" text :icon="Sparkles" @click.stop="openMitigate(row)">登记缓解</el-button><span v-else class="muted">—</span></template></el-table-column>
          </el-table>
        </div>
      </div>
      <EvidencePathPanel :item="selectedRisk" />
    </div>

    <div v-if="result?.cycles.length" class="cycle-band"><AlertTriangle :size="17" /><div><strong>检测到 {{ result.cycles.length }} 个环路，已跳过 {{ result.cycle_edges_skipped }} 条回访边</strong><span v-for="cycle in result.cycles" :key="cycle.join('-')" class="mono">{{ cycle.join(' → ') }}</span></div></div>
  </section>

  <el-dialog v-model="mitigateOpen" title="登记缓解措施" width="560px" destroy-on-close>
    <div v-if="mitigateForm.item" class="mitigate-target">{{ mitigateForm.item.allergen }} · {{ mitigateForm.item.source_step_code }} → {{ mitigateForm.item.target_step_code }} · 原始分数 {{ percent(mitigateForm.item.raw_score) }}</div>
    <el-form label-position="top">
      <el-form-item label="措施类型" required><el-select v-model="mitigateForm.measure_type" style="width: 100%"><el-option label="清洗" value="cleaning" /><el-option label="换线" value="line_change" /></el-select></el-form-item>
      <el-form-item label="完成日期（不能晚于提交日）" required><el-date-picker v-model="mitigateForm.completed_at" type="date" value-format="YYYY-MM-DD" :disabled-date="(date: Date) => date.getTime() > Date.now()" style="width: 100%" /></el-form-item>
      <el-form-item label="证据记录" required><el-input v-model="mitigateForm.evidence_note" type="textarea" :rows="3" maxlength="1000" show-word-limit placeholder="清洗验证、换线检查表或检测记录" /></el-form-item>
    </el-form>
    <template #footer><el-button @click="mitigateOpen = false">取消</el-button><el-button type="primary" :loading="saving" :disabled="!mitigateForm.completed_at || mitigateForm.evidence_note.trim().length < 4" @click="submitMitigation">提交复核</el-button></template>
  </el-dialog>
</template>

<style scoped>
.metric :deep(.risk-badge) { margin-top: 8px; }.matrix-error { min-height: 46px; display: flex; align-items: center; gap: 9px; padding: 10px 13px; color: #873a34; background: #faecea; border: 1px solid #dfb4b0; }.cell-sub { margin-top: 3px; font-size: 11px; }.cycle-band { display: flex; gap: 10px; padding: 12px 14px; color: #705212; background: #fff7e2; border: 1px solid #dfc986; }.cycle-band strong, .cycle-band span { display: block; }.cycle-band strong { font-size: 12px; }.cycle-band span { margin-top: 6px; color: #7c673a; }.mitigate-target { margin-bottom: 16px; padding: 10px 12px; background: var(--surface-muted); border: 1px solid var(--line); font: 12px "SFMono-Regular", Consolas, monospace; }
</style>
