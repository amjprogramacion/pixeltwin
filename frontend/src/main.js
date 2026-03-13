import { SelectFolder, Scan, CancelScan, DeleteFiles, OpenFile, GetHistory } from '../wailsjs/go/main/App.js'
import { EventsOn } from '../wailsjs/runtime/runtime.js'

// ── Estado global ─────────────────────────────────
let selectedFolder  = ''
let scanResults     = null
let selectedPaths   = new Set()
let activeFilter    = 'all'
let multiSelectMode = false   // true = clic en imagen selecciona; false = abre lightbox
let scanCancelled   = false   // true mientras se espera que Go confirme la cancelacion

// ── Versión desde .env (inyectada por Vite en build time) ───────────────
document.getElementById('appVersion').textContent = 'v' + import.meta.env.VITE_APP_VERSION
loadHistory()

// ── Referencias DOM ───────────────────────────────
const btnFolder      = document.getElementById('btnFolder')
const btnScan        = document.getElementById('btnScan')
const folderPath     = document.getElementById('folderPath')
const sliderSim      = document.getElementById('sliderSim')
const simVal         = document.getElementById('simVal')
const emptyState     = document.getElementById('emptyState')
const progressState  = document.getElementById('progressState')
const progressFill   = document.getElementById('progressFill')
const progressDone   = document.getElementById('progressDone')
const progressTotal  = document.getElementById('progressTotal')
const progressPct    = document.getElementById('progressPct')
const resultsArea    = document.getElementById('resultsArea')
const groupsList     = document.getElementById('groupsList')
const statsSection   = document.getElementById('statsSection')
const filterSection  = document.getElementById('filterSection')
const btnAutoSelect  = document.getElementById('btnAutoSelect')
const btnMultiSelect = document.getElementById('btnMultiSelect')
const btnCancel      = document.getElementById('btnCancel')
const btnClearSel    = document.getElementById('btnClearSel')
const btnDelete      = document.getElementById('btnDelete')
const delBadge       = document.getElementById('delBadge')
const resultsCount   = document.getElementById('resultsCount')
const modalOverlay   = document.getElementById('modalOverlay')
const modalMsg       = document.getElementById('modalMsg')
const modalCancel    = document.getElementById('modalCancel')
const modalConfirm   = document.getElementById('modalConfirm')
const historySection  = document.getElementById('historySection')
const historyToggle   = document.getElementById('historyToggle')
const historyList     = document.getElementById('historyList')
const lightbox        = document.getElementById('lightbox')
const lightboxImg     = document.getElementById('lightboxImg')
const lightboxClose   = document.getElementById('lightboxClose')
const lightboxName    = document.getElementById('lightboxName')
const lightboxMeta    = document.getElementById('lightboxMeta')
const lightboxCounter = document.getElementById('lightboxCounter')
const lightboxPrev    = document.getElementById('lightboxPrev')
const lightboxNext    = document.getElementById('lightboxNext')
const lightboxStrip   = document.getElementById('lightboxStrip')
const lightboxSpinner = document.getElementById('lightboxSpinner')

// Lista plana de todas las imágenes del grupo activo en el lightbox
let lightboxImages = []
let lightboxIndex  = 0

// ── Slider similitud ──────────────────────────────
sliderSim.addEventListener('input', () => {
  simVal.textContent = sliderSim.value + '%'
})

// ── Elegir carpeta ────────────────────────────────
btnFolder.addEventListener('click', async () => {
  const folder = await SelectFolder()
  if (!folder) return
  selectedFolder = folder
  folderPath.textContent = folder
  folderPath.title = folder
  btnScan.disabled = false
})

// ── Escanear ──────────────────────────────────────
btnScan.addEventListener('click', async () => {
  if (!selectedFolder) return
  const pct = parseInt(sliderSim.value)
  const threshold = Math.round((100 - pct) * 0.64)
  scanCancelled = false
  showProgress()
  try {
    const result = await Scan(selectedFolder, threshold)
    if (scanCancelled) { showEmpty(); return }
    scanResults = result
    showResults(result)
    loadHistory() // actualizar historial con el escaneo recién completado
  } catch (err) {
    // Cualquier error durante cancelación o error real → estado inicial
    showEmpty()
    if (!scanCancelled && !String(err).toLowerCase().includes('cancelado')) {
      alert('Error durante el escaneo: ' + err)
    }
    scanCancelled = false
  }
})

// ── Cancelar escaneo ─────────────────────────────
btnCancel.addEventListener('click', async () => {
  scanCancelled = true
  btnCancel.disabled    = true
  btnCancel.textContent = 'Parando…'
  await CancelScan()
  // showEmpty se llama desde el catch de Scan() cuando Go confirma la cancelación
})

// ── Progreso ──────────────────────────────────────
EventsOn('scan:progress', (data) => {
  progressFill.style.width  = data.percent + '%'
  progressDone.textContent  = data.done
  progressTotal.textContent = data.total
  progressPct.textContent   = data.percent + '%'
})

// ── Modo selección múltiple ───────────────────────
btnMultiSelect.addEventListener('click', () => {
  multiSelectMode = !multiSelectMode
  btnMultiSelect.classList.toggle('active', multiSelectMode)
  btnMultiSelect.textContent = multiSelectMode ? 'Selección múltiple ✓' : 'Selección múltiple'
  // Actualizar cursor en todas las tarjetas
  document.querySelectorAll('.img-card').forEach(el => {
    el.classList.toggle('multi-mode', multiSelectMode)
  })
})

// ── Estados de pantalla ───────────────────────────
function showEmpty() {
  emptyState.style.display    = 'flex'
  progressState.style.display = 'none'
  resultsArea.style.display   = 'none'
  btnScan.classList.remove('scanning')
  btnScan.disabled    = !selectedFolder
  btnScan.textContent = 'Escanear'
  btnCancel.style.display = 'none'
}

function showProgress() {
  emptyState.style.display    = 'none'
  progressState.style.display = 'flex'
  resultsArea.style.display   = 'none'
  progressFill.style.width    = '0%'
  progressDone.textContent    = '0'
  progressTotal.textContent   = '?'
  progressPct.textContent     = '0%'
  btnScan.disabled    = true
  btnScan.classList.add('scanning')
  btnScan.textContent = 'Escaneando…'
  btnCancel.disabled    = false
  btnCancel.textContent = 'Parar escaneo'
  btnCancel.style.display = 'block'
}

function showResults(result) {
  emptyState.style.display    = 'none'
  progressState.style.display = 'none'
  resultsArea.style.display   = 'block'
  btnScan.classList.remove('scanning')
  btnScan.disabled    = false
  btnScan.textContent = 'Escanear de nuevo'
  btnCancel.style.display = 'none'

  document.getElementById('stFound').textContent   = result.totalScanned.toLocaleString()
  document.getElementById('stGroups').textContent  = result.groupCount
  document.getElementById('stReclaim').textContent = result.reclaimable
  document.getElementById('stTime').textContent    = result.duration
  statsSection.style.display  = 'flex'
  filterSection.style.display = 'flex'

  selectedPaths.clear()
  updateDeleteBtn()
  renderGroups(result.groups)
}

// ── Render grupos ─────────────────────────────────
function renderGroups(groups) {
  const filtered = groups.filter(g => {
    if (activeFilter === 'all')     return true
    if (activeFilter === 'exact')   return g.groupType === 'exact'
    if (activeFilter === 'similar') return g.groupType === 'similar'
    return true
  })

  resultsCount.textContent = filtered.length + ' grupo' + (filtered.length !== 1 ? 's' : '')
  groupsList.innerHTML = ''

  if (filtered.length === 0) {
    groupsList.innerHTML = '<p style="color:var(--text3);padding:32px;text-align:center">No hay grupos con este filtro</p>'
    return
  }

  filtered.forEach(group => {
    const card = document.createElement('div')
    card.className = 'group-card'
    card.dataset.groupId = group.id

    const badgeClass = group.groupType === 'exact' ? 'badge-exact' : 'badge-similar'
    const badgeText  = group.groupType === 'exact'
      ? 'Exacto'
      : 'Similar ' + Math.round(group.similarity) + '%'

    card.innerHTML = `
      <div class="group-header">
        <span class="group-num">GRUPO ${group.id}</span>
        <span class="group-badge ${badgeClass}">${badgeText}</span>
        <span class="group-meta">
          ${group.images.length} archivos ·
          <span class="group-waste">${group.wasteSize} recuperables</span>
        </span>
      </div>
      <div class="img-grid">
        ${group.images.map(img => renderImgCard(img)).join('')}
      </div>
    `
    groupsList.appendChild(card)
  })

  // Eventos en tarjetas
  groupsList.querySelectorAll('.img-card').forEach(el => {
    if (multiSelectMode) el.classList.add('multi-mode')

    // Clic en la imagen (zona superior) → lightbox o selección
    el.querySelector('.img-thumb-wrap').addEventListener('click', () => {
      if (multiSelectMode) {
        toggleSelect(el, el.dataset.path)
      } else {
        // Recoger todas las imágenes del grupo para poder navegar entre ellas
        const groupCard  = el.closest('.group-card')
        const groupId    = parseInt(groupCard.dataset.groupId)
        const groupData  = scanResults.groups.find(g => g.id === groupId)
        const groupImgs  = groupData ? groupData.images : []
        openLightbox(el.dataset.path, groupImgs.length ? groupImgs : [{ path: el.dataset.path, filename: el.dataset.filename, sizeFmt: '', width: 0, height: 0, nameDate: '' }])
      }
    })

    // Clic en el checkbox (siempre selecciona, en cualquier modo)
    el.querySelector('.img-checkbox').addEventListener('click', (e) => {
      e.stopPropagation()
      toggleSelect(el, el.dataset.path)
    })

    // Botón abrir archivo
    el.querySelector('.open-btn').addEventListener('click', (e) => {
      e.stopPropagation()
      OpenFile(el.dataset.path)
    })
  })
}

function renderImgCard(img) {
  const originBadge = img.isOrigin
    ? '<span class="origin-badge">Original sugerido</span>'
    : ''
  const dateLine = img.nameDate
    ? `<div class="img-date">📅 ${img.nameDate}</div>`
    : ''
  const isSelected = selectedPaths.has(img.path)

  return `
    <div class="img-card ${isSelected ? 'selected' : ''} ${multiSelectMode ? 'multi-mode' : ''}"
         data-path="${escHtml(img.path)}"
         data-filename="${escHtml(img.filename)}"
         data-meta="${escHtml(img.sizeFmt + ' · ' + img.width + '×' + img.height)}">

      <!-- Zona clickable para lightbox / selección múltiple -->
      <div class="img-thumb-wrap">
        <img class="img-thumb"
             src="/thumb?path=${encodeURIComponent(img.path)}"
             alt="${escHtml(img.filename)}"
             onerror="this.style.opacity='0.2'"/>
        <button class="open-btn" data-path="${escHtml(img.path)}">Abrir</button>
        <!-- Icono de lupa visible en modo normal como hint -->
        <div class="zoom-hint">
          <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.8"
               stroke-linecap="round" stroke-linejoin="round">
            <circle cx="6.5" cy="6.5" r="4.5"/>
            <line x1="10.5" y1="10.5" x2="14" y2="14"/>
            <line x1="6.5" y1="4.5" x2="6.5" y2="8.5"/>
            <line x1="4.5" y1="6.5" x2="8.5" y2="6.5"/>
          </svg>
        </div>
      </div>

      <!-- Info y checkbox en la parte inferior -->
      <div class="img-footer">
        <div class="img-info">
          <div class="img-name" title="${escHtml(img.path)}">${escHtml(img.filename)}</div>
          <div class="img-meta-line">${img.sizeFmt} · ${img.width}×${img.height}</div>
          ${dateLine}
          ${originBadge}
        </div>
        <!-- Checkbox siempre visible en esquina inferior derecha -->
        <div class="img-checkbox ${isSelected ? 'checked' : ''}">
          <svg viewBox="0 0 12 12" fill="none" stroke="currentColor" stroke-width="2.2"
               stroke-linecap="round" stroke-linejoin="round">
            <polyline points="2,6 5,9 10,3"/>
          </svg>
        </div>
      </div>
    </div>
  `
}

// ── Lightbox ──────────────────────────────────────

// Abre el lightbox posicionado en la imagen clickada dentro de su grupo
function openLightbox(path, groupImages) {
  lightboxImages = groupImages  // array de ImageDTO del grupo
  lightboxIndex  = groupImages.findIndex(img => img.path === path)
  if (lightboxIndex < 0) lightboxIndex = 0

  renderLightboxStrip()
  navigateLightbox(lightboxIndex)

  lightbox.style.display       = 'flex'
  document.body.style.overflow = 'hidden'
}

// Navega a un índice concreto dentro del grupo
function navigateLightbox(idx) {
  if (!lightboxImages.length) return
  // Wrap alrededor
  lightboxIndex = (idx + lightboxImages.length) % lightboxImages.length
  const img = lightboxImages[lightboxIndex]

  // Spinner mientras carga
  lightboxSpinner.style.display = 'block'
  lightboxImg.style.opacity     = '0'

  lightboxImg.onload = () => {
    lightboxSpinner.style.display = 'none'
    lightboxImg.style.opacity     = '1'
  }
  lightboxImg.onerror = () => {
    lightboxSpinner.style.display = 'none'
    lightboxImg.style.opacity     = '0.3'
  }

  lightboxImg.src              = '/thumb?path=' + encodeURIComponent(img.path) + '&size=1200'
  lightboxName.textContent     = img.filename
  lightboxMeta.textContent     = img.sizeFmt + ' · ' + img.width + '×' + img.height + (img.nameDate ? ' · ' + img.nameDate : '')
  lightboxCounter.textContent  = (lightboxIndex + 1) + ' / ' + lightboxImages.length

  // Botones de navegación: ocultar si solo hay una imagen
  const showNav = lightboxImages.length > 1
  lightboxPrev.style.display = showNav ? 'flex' : 'none'
  lightboxNext.style.display = showNav ? 'flex' : 'none'

  // Resaltar miniatura activa en la tira
  lightboxStrip.querySelectorAll('.strip-thumb').forEach((el, i) => {
    el.classList.toggle('active', i === lightboxIndex)
  })
}

// Construye la tira de miniaturas del grupo
function renderLightboxStrip() {
  lightboxStrip.innerHTML = ''
  if (lightboxImages.length <= 1) {
    lightboxStrip.style.display = 'none'
    return
  }
  lightboxStrip.style.display = 'flex'
  lightboxImages.forEach((img, i) => {
    const el = document.createElement('div')
    el.className = 'strip-thumb' + (i === lightboxIndex ? ' active' : '')
    el.innerHTML = `<img src="/thumb?path=${encodeURIComponent(img.path)}" alt=""/>`
    el.addEventListener('click', () => navigateLightbox(i))
    lightboxStrip.appendChild(el)
  })
}

function closeLightbox() {
  lightbox.style.display       = 'none'
  lightboxImg.src              = ''
  lightboxImages               = []
  document.body.style.overflow = ''
}

lightboxClose.addEventListener('click', closeLightbox)
lightboxPrev.addEventListener('click',  () => navigateLightbox(lightboxIndex - 1))
lightboxNext.addEventListener('click',  () => navigateLightbox(lightboxIndex + 1))

lightbox.addEventListener('click', (e) => {
  // Cerrar si el clic cae en el overlay oscuro o fuera del contenedor de imagen,
  // pero no si es en los botones de nav, la tira, o la topbar
  const insideContent = e.target.closest(
    '.lightbox-img-container, .lightbox-nav, .lightbox-topbar, .lightbox-strip, .lightbox-close'
  )
  if (!insideContent) closeLightbox()
})

document.addEventListener('keydown', (e) => {
  if (lightbox.style.display === 'none') return
  if (e.key === 'Escape')      closeLightbox()
  if (e.key === 'ArrowRight' || e.key === 'ArrowDown')
    navigateLightbox(lightboxIndex + 1)
  if (e.key === 'ArrowLeft'  || e.key === 'ArrowUp')
    navigateLightbox(lightboxIndex - 1)
})

// ── Selección ─────────────────────────────────────
function toggleSelect(el, path) {
  const cb = el.querySelector('.img-checkbox')
  if (selectedPaths.has(path)) {
    selectedPaths.delete(path)
    el.classList.remove('selected')
    cb.classList.remove('checked')
  } else {
    selectedPaths.add(path)
    el.classList.add('selected')
    cb.classList.add('checked')
  }
  updateDeleteBtn()
}

function updateDeleteBtn() {
  const n = selectedPaths.size
  btnDelete.disabled   = n === 0
  delBadge.textContent = n
}

btnAutoSelect.addEventListener('click', () => {
  if (!scanResults) return
  const groups = scanResults.groups.filter(g =>
    activeFilter === 'all' || g.groupType === activeFilter
  )
  groups.forEach(group => {
    group.images.forEach(img => {
      if (!img.isOrigin && !selectedPaths.has(img.path)) {
        selectedPaths.add(img.path)
        const el = groupsList.querySelector(`.img-card[data-path="${CSS.escape(img.path)}"]`)
        if (el) {
          el.classList.add('selected')
          el.querySelector('.img-checkbox').classList.add('checked')
        }
      }
    })
  })
  updateDeleteBtn()
})

btnClearSel.addEventListener('click', () => {
  selectedPaths.clear()
  groupsList.querySelectorAll('.img-card.selected').forEach(el => {
    el.classList.remove('selected')
    el.querySelector('.img-checkbox').classList.remove('checked')
  })
  updateDeleteBtn()
})

// ── Filtros ───────────────────────────────────────
document.querySelectorAll('.filter-btn').forEach(btn => {
  btn.addEventListener('click', () => {
    document.querySelectorAll('.filter-btn').forEach(b => b.classList.remove('active'))
    btn.classList.add('active')
    activeFilter = btn.dataset.filter
    if (scanResults) renderGroups(scanResults.groups)
  })
})

// ── Borrar ────────────────────────────────────────
btnDelete.addEventListener('click', () => {
  if (selectedPaths.size === 0) return
  modalMsg.innerHTML = `¿Mover <strong>${selectedPaths.size} archivo${selectedPaths.size !== 1 ? 's' : ''}</strong> a la papelera?`
  modalOverlay.style.display = 'flex'
})

modalCancel.addEventListener('click', () => { modalOverlay.style.display = 'none' })

modalConfirm.addEventListener('click', async () => {
  modalOverlay.style.display = 'none'
  const paths  = [...selectedPaths]
  const failed = await DeleteFiles(paths)

  paths.forEach(p => {
    if (!failed.includes(p)) {
      const el = groupsList.querySelector(`.img-card[data-path="${CSS.escape(p)}"]`)
      if (el) el.remove()
    }
  })

  selectedPaths.clear()
  updateDeleteBtn()

  groupsList.querySelectorAll('.group-card').forEach(card => {
    if (card.querySelectorAll('.img-card').length < 2) card.remove()
  })

  const remaining = groupsList.querySelectorAll('.group-card').length
  resultsCount.textContent = remaining + ' grupo' + (remaining !== 1 ? 's' : '')

  if (failed.length > 0) {
    alert(`No se pudieron mover ${failed.length} archivo(s). Puede que estén en uso.`)
  }
})

modalOverlay.addEventListener('click', (e) => {
  if (e.target === modalOverlay) modalOverlay.style.display = 'none'
})

// ── Historial ─────────────────────────────────────
async function loadHistory() {
  const entries = await GetHistory()
  if (!entries || entries.length === 0) {
    historySection.style.display = 'none'
    return
  }

  historySection.style.display = 'block'
  historyList.innerHTML = entries.map(e => `
    <li class="history-item" data-folder="${escHtml(e.folder)}">
      <div class="history-item-folder" title="${escHtml(e.folder)}">${escHtml(e.folder)}</div>
      <div class="history-item-meta">
        <span>${escHtml(e.scannedAt)}</span>
        <span class="history-item-groups">${e.groups} grupo${e.groups !== 1 ? 's' : ''}</span>
      </div>
    </li>
  `).join('')

  historyList.querySelectorAll('.history-item').forEach(el => {
    el.addEventListener('click', () => {
      const folder = el.dataset.folder
      selectedFolder = folder
      folderPath.textContent = folder
      folderPath.title = folder
      btnScan.disabled = false
      // Lanzar escaneo automáticamente
      btnScan.click()
    })
  })
}

historyToggle.addEventListener('click', () => {
  const isOpen = historyToggle.classList.toggle('open')
  historyList.style.display = isOpen ? 'block' : 'none'
})

// ── Util ──────────────────────────────────────────
function escHtml(str) {
  if (!str) return ''
  return str.replace(/&/g,'&amp;').replace(/"/g,'&quot;').replace(/</g,'&lt;').replace(/>/g,'&gt;')
}
