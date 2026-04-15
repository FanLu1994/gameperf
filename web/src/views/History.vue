<template>
  <div class="history">
    <el-card>
      <template #header>
        <div style="display:flex;justify-content:space-between;align-items:center">
          <span>📚 历史记录</span>
          <el-button type="primary" @click="$router.push('/compare')">对比分析 →</el-button>
        </div>
      </template>
      <el-table :data="sessions" stripe v-loading="loading">
        <el-table-column prop="name" label="标签" width="180">
          <template #default="{ row }"><strong>{{ row.name || '-' }}</strong></template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="row.status === 'running' ? 'success' : 'info'" size="small">{{ row.status }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="start_time" label="时间" width="200" />
        <el-table-column label="CPU Avg/Max/P95">
          <template #default="{ row }">
            <span v-if="summaries[row.id]">
              {{ summaries[row.id].avg_cpu?.toFixed(1) }} / {{ summaries[row.id].max_cpu?.toFixed(1) }} / {{ summaries[row.id].p95_cpu?.toFixed(1) }} %
            </span>
          </template>
        </el-table-column>
        <el-table-column label="内存 Max/P95" width="160">
          <template #default="{ row }">
            <span v-if="summaries[row.id]">
              {{ summaries[row.id].max_memory?.toFixed(0) }} / {{ summaries[row.id].p95_memory?.toFixed(0) }} MB
            </span>
          </template>
        </el-table-column>
        <el-table-column label="GPU/FPS" width="160">
          <template #default="{ row }">
            <span v-if="summaries[row.id]">
              <span v-if="summaries[row.id].avg_gpu">GPU {{ summaries[row.id].avg_gpu?.toFixed(0) }}%</span>
              <span v-if="summaries[row.id].avg_fps"> FPS {{ summaries[row.id].avg_fps?.toFixed(1) }}</span>
            </span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="250" fixed="right">
          <template #default="{ row }">
            <el-button size="small" @click="showDetail(row.id)">详情</el-button>
            <el-button size="small" @click="showFrameAnalysis(row.id)">帧分析</el-button>
            <el-button size="small" type="primary" @click="$router.push('/compare?highlight=' + row.id)">对比</el-button>
            <el-button size="small" type="danger" @click="handleDelete(row.id)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 详情弹窗 -->
    <el-dialog v-model="detailVisible" :title="detailData?.session?.name || '详情'" width="1000px">
      <div v-if="detailData">
        <el-descriptions :column="4" border size="small">
          <el-descriptions-item label="采样数">{{ detailData.sample_count }}</el-descriptions-item>
          <el-descriptions-item label="时长">{{ detailData.duration?.toFixed(0) }}s</el-descriptions-item>
          <!-- CPU -->
          <el-descriptions-item label="CPU Avg">{{ detailData.avg_cpu?.toFixed(1) }}%</el-descriptions-item>
          <el-descriptions-item label="CPU P95/P99">{{ detailData.p95_cpu?.toFixed(1) }} / {{ detailData.p99_cpu?.toFixed(1) }}%</el-descriptions-item>
          <!-- Memory -->
          <el-descriptions-item label="内存 Max">{{ detailData.max_memory?.toFixed(0) }} MB</el-descriptions-item>
          <el-descriptions-item label="内存 P95/P99">{{ detailData.p95_memory?.toFixed(0) }} / {{ detailData.p99_memory?.toFixed(0) }} MB</el-descriptions-item>
          <!-- GPU -->
          <el-descriptions-item label="GPU Avg" v-if="detailData.avg_gpu">{{ detailData.avg_gpu?.toFixed(1) }}%</el-descriptions-item>
          <el-descriptions-item label="GPU Max Temp" v-if="detailData.max_gpu_temp">{{ detailData.max_gpu_temp?.toFixed(0) }}℃</el-descriptions-item>
          <el-descriptions-item label="GPU Max Mem" v-if="detailData.max_gpu_mem">{{ detailData.max_gpu_mem?.toFixed(0) }} MB</el-descriptions-item>
          <el-descriptions-item label="GPU Avg Power" v-if="detailData.avg_gpu_power">{{ detailData.avg_gpu_power?.toFixed(1) }} W</el-descriptions-item>
          <!-- FPS -->
          <el-descriptions-item label="FPS Avg" v-if="detailData.avg_fps">{{ detailData.avg_fps?.toFixed(1) }}</el-descriptions-item>
          <el-descriptions-item label="FPS Min/P1">{{ detailData.min_fps?.toFixed(1) }} / {{ detailData.p1_fps?.toFixed(1) }}</el-descriptions-item>
          <el-descriptions-item label="FPS 稳定性" v-if="detailData.fps_stability">{{ (detailData.fps_stability * 100).toFixed(1) }}%</el-descriptions-item>
          <el-descriptions-item label="帧时间 P95/P99" v-if="detailData.avg_frame_time">{{ detailData.p95_frame_time?.toFixed(2) }} / {{ detailData.p99_frame_time?.toFixed(2) }} ms</el-descriptions-item>
          <!-- Jank -->
          <el-descriptions-item label="卡顿帧" v-if="detailData.total_jank_count">{{ detailData.total_jank_count }}</el-descriptions-item>
          <el-descriptions-item label="卡顿率" v-if="detailData.avg_stutter_rate">{{ detailData.avg_stutter_rate?.toFixed(2) }}%</el-descriptions-item>
          <!-- Disk -->
          <el-descriptions-item label="磁盘读峰值">{{ formatBPS(detailData.max_disk_read_bps) }}</el-descriptions-item>
          <el-descriptions-item label="磁盘写峰值">{{ formatBPS(detailData.max_disk_write_bps) }}</el-descriptions-item>
          <!-- Net -->
          <el-descriptions-item label="网络发峰值">{{ formatBPS(detailData.max_net_sent_bps) }}</el-descriptions-item>
          <el-descriptions-item label="网络收峰值">{{ formatBPS(detailData.max_net_recv_bps) }}</el-descriptions-item>
          <!-- Other -->
          <el-descriptions-item label="最大线程数">{{ detailData.max_threads }}</el-descriptions-item>
          <el-descriptions-item label="最大句柄数">{{ detailData.max_handle_count }}</el-descriptions-item>
        </el-descriptions>
        <div ref="detailChart" class="detail-chart"></div>
      </div>
    </el-dialog>

    <!-- 帧时间分析弹窗 -->
    <el-dialog v-model="frameVisible" title="🎞️ 帧时间分布分析" width="900px">
      <div v-if="frameData">
        <el-descriptions :column="3" border size="small" style="margin-bottom:16px">
          <el-descriptions-item label="总帧数">{{ frameData.frame_count }}</el-descriptions-item>
          <el-descriptions-item label="卡顿帧">{{ frameData.jank_frames?.length || 0 }}</el-descriptions-item>
          <el-descriptions-item label="卡顿段">{{ frameData.stutter_sections?.length || 0 }}</el-descriptions-item>
        </el-descriptions>
        <div ref="histogramChart" class="detail-chart"></div>
        <!-- 卡顿段列表 -->
        <el-table v-if="frameData.stutter_sections?.length" :data="frameData.stutter_sections" stripe max-height="200" style="margin-top:12px">
          <el-table-column label="开始" width="120"><template #default="{ row }">{{ row.start_elapsed?.toFixed(1) }}s</template></el-table-column>
          <el-table-column label="结束" width="120"><template #default="{ row }">{{ row.end_elapsed?.toFixed(1) }}s</template></el-table-column>
          <el-table-column label="持续" width="100"><template #default="{ row }">{{ row.duration?.toFixed(2) }}s</template></el-table-column>
          <el-table-column label="卡顿帧数"><template #default="{ row }">{{ row.frame_count }}</template></el-table-column>
        </el-table>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted, nextTick } from 'vue'
import * as echarts from 'echarts'
import { listSessions, getSummary, getSamples, deleteSession, getFrameAnalysis } from '../api'
import { ElMessage } from 'element-plus'

const sessions = ref([])
const summaries = ref({})
const loading = ref(false)
const detailVisible = ref(false)
const detailData = ref(null)
const detailChart = ref(null)
let detailInstance = null

const frameVisible = ref(false)
const frameData = ref(null)
const histogramChart = ref(null)
let histInstance = null

function formatBPS(bps) {
  if (!bps) return '-'
  if (bps > 1048576) return (bps / 1048576).toFixed(1) + ' MB/s'
  if (bps > 1024) return (bps / 1024).toFixed(1) + ' KB/s'
  return bps.toFixed(0) + ' B/s'
}

async function loadSessions() {
  loading.value = true
  try {
    const { data } = await listSessions()
    sessions.value = data
    for (const s of data.slice(0, 20)) {
      getSummary(s.id).then(({ data }) => { summaries.value[s.id] = data }).catch(() => {})
    }
  } catch (e) { console.error(e) }
  loading.value = false
}

async function showDetail(id) {
  try {
    const { data: summary } = await getSummary(id)
    const { data: samples } = await getSamples(id)
    detailData.value = { ...summary, samples }
    detailVisible.value = true
    await nextTick()
    if (!detailInstance) detailInstance = echarts.init(detailChart.value, 'dark')

    const times = samples.map(s => `${Math.floor(s.elapsed/60)}:${String(Math.floor(s.elapsed%60)).padStart(2,'0')}`)
    const series = [
      { name: 'CPU%', data: samples.map(s => s.cpu.toFixed(1)), color: '#00d4ff' },
      { name: '内存MB', data: samples.map(s => s.memory.toFixed(1)), color: '#36d399' },
      { name: 'GPU%', data: samples.map(s => s.gpu_util.toFixed(1)), color: '#a855f7' },
    ].filter(s => samples.some(x => parseFloat(s.data[samples.indexOf(x)] || 0) > 0))

    detailInstance.setOption({
      backgroundColor: 'transparent', tooltip: { trigger: 'axis' },
      legend: { textStyle: { color: '#999' } },
      grid: { top: 40, right: 60, bottom: 30, left: 60 },
      xAxis: { type: 'category', data: times, axisLabel: { color: '#666' } },
      yAxis: { type: 'value', axisLabel: { color: '#666' }, splitLine: { lineStyle: { color: '#1a1a3a' } } },
      series: series.map(s => ({ name: s.name, type: 'line', smooth: true, showSymbol: false, data: s.data, itemStyle: { color: s.color } })),
    })
  } catch { ElMessage.error('加载失败') }
}

async function showFrameAnalysis(id) {
  try {
    const { data } = await getFrameAnalysis(id)
    frameData.value = data
    frameVisible.value = true
    await nextTick()
    if (!histInstance) histInstance = echarts.init(histogramChart.value, 'dark')

    if (data.histogram?.length) {
      histInstance.setOption({
        backgroundColor: 'transparent',
        tooltip: { trigger: 'axis' },
        grid: { top: 20, right: 20, bottom: 30, left: 60 },
        xAxis: { type: 'category', data: data.histogram.map(b => b.range_start.toFixed(1) + '-' + b.range_end.toFixed(1)), axisLabel: { color: '#666', rotate: 45 } },
        yAxis: { type: 'value', name: '帧数', axisLabel: { color: '#666' }, splitLine: { lineStyle: { color: '#1a1a3a' } } },
        series: [{
          type: 'bar', data: data.histogram.map(b => b.count),
          itemStyle: { color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [{ offset: 0, color: '#00d4ff' }, { offset: 1, color: '#36d399' }]) },
          barWidth: '80%',
        }],
      })
    }
  } catch { ElMessage.error('帧分析失败') }
}

async function handleDelete(id) {
  await deleteSession(id); ElMessage.success('已删除'); loadSessions()
}

onMounted(loadSessions)
</script>

<style scoped>
.detail-chart { height: 350px; margin-top: 16px; }
</style>
