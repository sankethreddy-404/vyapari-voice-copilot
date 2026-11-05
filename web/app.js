"use strict";

const state = {
  dashboard: null,
  health: null,
  recording: false,
  audioContext: null,
  stream: null,
  sourceNode: null,
  analyser: null,
  processor: null,
  silentGain: null,
  chunks: [],
  sampleRate: 48000,
  animationFrame: 0,
  timerInterval: 0,
  recordingStartedAt: 0,
  maxRecordingTimer: 0,
  currentAudioURL: "",
  lastSpeechText: "",
  lastSpeechLanguage: "te-IN",
  installPrompt: null,
  toastTimer: 0,
};

const $ = (id) => document.getElementById(id);
const currency = new Intl.NumberFormat("en-IN", {
  style: "currency",
  currency: "INR",
  maximumFractionDigits: 0,
});
const number = new Intl.NumberFormat("en-IN", { maximumFractionDigits: 1 });

window.addEventListener("DOMContentLoaded", init);

async function init() {
  restoreSettings();
  bindEvents();
  drawIdleWaveform();
  await Promise.allSettled([loadHealth(), loadDashboard()]);
  registerServiceWorker();
}

function restoreSettings() {
  $("responseLanguage").value = localStorage.getItem("vyapari.responseLanguage") || "auto";
  $("autoSpeak").checked = localStorage.getItem("vyapari.autoSpeak") !== "false";
}

function bindEvents() {
  $("commandForm").addEventListener("submit", (event) => {
    event.preventDefault();
    const text = $("commandInput").value.trim();
    if (text) executeCommand(text);
  });

  $("micButton").addEventListener("click", () => {
    if (state.recording) {
      stopRecording(true);
    } else {
      startRecording();
    }
  });

  $("responseLanguage").addEventListener("change", () => {
    localStorage.setItem("vyapari.responseLanguage", $("responseLanguage").value);
    renderSuggestions(defaultSuggestions($("responseLanguage").value));
  });

  $("autoSpeak").addEventListener("change", () => {
    localStorage.setItem("vyapari.autoSpeak", String($("autoSpeak").checked));
  });

  $("replayButton").addEventListener("click", () => {
    if (state.lastSpeechText) {
      speakResponse(state.lastSpeechText, state.lastSpeechLanguage, true);
    }
  });

  $("inventorySearch").addEventListener("input", renderInventory);
  $("inventoryBody").addEventListener("click", handleInventoryAction);
  $("undoButton").addEventListener("click", undoLastAction);
  $("resetButton").addEventListener("click", resetDemo);

  $("addProductButton").addEventListener("click", openProductDialog);
  $("closeProductDialog").addEventListener("click", closeProductDialog);
  $("cancelProductButton").addEventListener("click", closeProductDialog);
  $("productForm").addEventListener("submit", addProduct);

  $("installButton").addEventListener("click", installApp);
  window.addEventListener("beforeinstallprompt", (event) => {
    event.preventDefault();
    state.installPrompt = event;
    $("installButton").classList.remove("hidden");
  });

  window.addEventListener("resize", debounce(() => {
    if (state.dashboard) drawSalesChart(state.dashboard.salesTrend || []);
  }, 140));
}

async function loadHealth() {
  try {
    const data = await requestJSON("/api/health");
    state.health = data;
    const configured = Boolean(data.gnani && data.gnani.configured);
    const status = $("apiStatus");
    status.className = `status-pill ${configured ? "status-connected" : "status-offline"}`;
    status.querySelector("span:last-child").textContent = configured
      ? `${data.gnani.sttModel} + ${data.gnani.ttsModel}`
      : "Add API key for voice";
    if (!configured) {
      $("sttMeta").textContent = "Typed commands work now. Add GNANI_API_KEY to enable Gnani voice.";
    }
  } catch (error) {
    const status = $("apiStatus");
    status.className = "status-pill status-offline";
    status.querySelector("span:last-child").textContent = "Server unavailable";
    showToast(error.message, true);
  }
}

async function loadDashboard() {
  try {
    const data = await requestJSON("/api/dashboard");
    state.dashboard = data.dashboard;
    renderDashboard();
  } catch (error) {
    showToast(error.message, true);
  }
}

function renderDashboard() {
  if (!state.dashboard) return;
  const { metrics } = state.dashboard;
  $("todayRevenue").textContent = currency.format(metrics.todayRevenue || 0);
  $("todayOrders").textContent = `${metrics.todayOrders || 0} orders`;
  $("todayProfit").textContent = currency.format(metrics.todayProfit || 0);
  $("itemsSold").textContent = number.format(metrics.todayItemsSold || 0);
  $("lowStockCount").textContent = number.format(metrics.lowStockCount || 0);
  $("inventoryValue").textContent = currency.format(metrics.inventoryValue || 0);

  const margin = metrics.todayRevenue > 0 ? (metrics.todayProfit / metrics.todayRevenue) * 100 : 0;
  $("profitMargin").textContent = `${number.format(margin)}% gross margin`;

  renderInventory();
  renderReorderPlan();
  renderTopProducts();
  renderActivities();
  drawSalesChart(state.dashboard.salesTrend || []);
}

function renderInventory() {
  const body = $("inventoryBody");
  if (!state.dashboard) {
    body.innerHTML = "";
    return;
  }

  const query = $("inventorySearch").value.trim().toLowerCase();
  const products = (state.dashboard.products || []).filter((product) => {
    const haystack = `${product.name} ${product.category} ${(product.aliases || []).join(" ")}`.toLowerCase();
    return !query || haystack.includes(query);
  });

  if (!products.length) {
    body.innerHTML = `<tr><td colspan="6"><div class="empty-state">No matching products found.</div></td></tr>`;
    return;
  }

  body.innerHTML = products.map((product) => {
    const low = product.stock <= product.lowStockThreshold;
    const margin = product.sellPrice > 0
      ? ((product.sellPrice - product.costPrice) / product.sellPrice) * 100
      : 0;
    return `
      <tr>
        <td>
          <div class="product-cell">
            <span class="product-avatar">${escapeHTML(product.name.slice(0, 1).toUpperCase())}</span>
            <div><strong>${escapeHTML(product.name)}</strong><small>${escapeHTML(product.category)}</small></div>
          </div>
        </td>
        <td><span class="stock-number">${number.format(product.stock)}</span> ${escapeHTML(product.unit)}</td>
        <td><span class="stock-badge ${low ? "stock-low" : "stock-good"}">${low ? "Low stock" : "Healthy"}</span></td>
        <td>${currency.format(product.sellPrice)}</td>
        <td class="margin-positive">${number.format(margin)}%</td>
        <td>
          <div class="quick-actions">
            <button class="mini-button" type="button" data-product-id="${escapeHTML(product.id)}" data-action="sale" data-quantity="1">Sell 1</button>
            <button class="mini-button" type="button" data-product-id="${escapeHTML(product.id)}" data-action="restock" data-quantity="5">+5 stock</button>
          </div>
        </td>
      </tr>`;
  }).join("");
}

function renderReorderPlan() {
  const container = $("reorderList");
  const items = state.dashboard?.reorderPlan || [];
  if (!items.length) {
    container.innerHTML = `<div class="empty-state"><div><strong>Stock levels look healthy.</strong><br>No reorder is currently recommended.</div></div>`;
    return;
  }

  container.innerHTML = items.slice(0, 5).map((item) => {
    const days = item.daysOfCover >= 999 ? "No recent demand" : `${number.format(item.daysOfCover)} days cover`;
    return `
      <div class="reorder-item">
        <div>
          <div class="reorder-item-head">
            <strong>${escapeHTML(item.productName)}</strong>
            <span class="priority-badge priority-${escapeHTML(item.priority)}">${escapeHTML(item.priority)}</span>
          </div>
          <p>${escapeHTML(days)} · ${escapeHTML(item.reason)}</p>
        </div>
        <div class="reorder-qty">
          <strong>+${number.format(item.recommendedQty)}</strong>
          <small>${currency.format(item.estimatedCost)}</small>
        </div>
      </div>`;
  }).join("");
}

function renderTopProducts() {
  const container = $("topProducts");
  const items = state.dashboard?.topProducts || [];
  if (!items.length) {
    container.innerHTML = `<div class="empty-state">Record a sale to build the leaderboard.</div>`;
    return;
  }
  const maxQuantity = Math.max(...items.map((item) => item.quantity), 1);
  container.innerHTML = items.map((item, index) => `
    <div class="top-product-row">
      <span class="top-rank">${index + 1}</span>
      <div class="top-product-info">
        <strong>${escapeHTML(item.productName)}</strong>
        <small>${currency.format(item.revenue)} revenue</small>
        <progress class="progress-track" max="${maxQuantity}" value="${item.quantity}" aria-label="${escapeHTML(item.productName)} sales"></progress>
      </div>
      <span class="top-quantity">${number.format(item.quantity)}</span>
    </div>`).join("");
}

function renderActivities() {
  const container = $("activityList");
  const activities = state.dashboard?.recentActivity || [];
  if (!activities.length) {
    container.innerHTML = `<div class="empty-state">No activity yet.</div>`;
    return;
  }
  container.innerHTML = activities.slice(0, 8).map((activity) => `
    <div class="activity-item">
      <span class="activity-dot"></span>
      <div><strong>${escapeHTML(activity.message)}</strong><small>${escapeHTML(activity.kind)}</small></div>
      <small>${relativeTime(activity.createdAt)}</small>
    </div>`).join("");
}

function renderSuggestions(suggestions) {
  const container = $("suggestionChips");
  container.innerHTML = (suggestions || []).map((suggestion) =>
    `<button class="suggestion-chip" type="button" data-command="${escapeHTML(suggestion)}">${escapeHTML(suggestion)}</button>`
  ).join("");
  container.querySelectorAll("[data-command]").forEach((button) => {
    button.addEventListener("click", () => {
      const command = button.dataset.command || "";
      $("commandInput").value = command;
      executeCommand(command);
    });
  });
}

function defaultSuggestions(language) {
  if (language === "te-IN" || language === "auto") {
    return [
      "Basmati rice stock entha undi?",
      "Basmati rice rendu packets sale ayindi",
      "Low stock products cheppu",
      "Ee roju sales entha?",
    ];
  }
  return [
    "How much Basmati Rice stock is available?",
    "Sell two packets of Basmati Rice",
    "Show low-stock products",
    "Give me a business briefing",
  ];
}

async function executeCommand(text) {
  text = String(text || "").trim();
  if (!text) return;

  setCommandBusy(true);
  $("transcriptText").textContent = text;
  $("responseText").textContent = "Understanding the command and checking shop data...";
  try {
    const data = await requestJSON("/api/command", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        text,
        responseLanguage: $("responseLanguage").value,
      }),
    });

    $("responseText").textContent = data.message;
    $("commandInput").value = "";
    state.lastSpeechText = data.speechText || data.message;
    state.lastSpeechLanguage = data.language || "te-IN";
    state.dashboard = data.dashboard;
    renderDashboard();
    renderSuggestions(data.suggestions || defaultSuggestions(data.language));

    if (data.changed) showToast("Shop data updated successfully.");
    if ($("autoSpeak").checked && state.lastSpeechText) {
      await speakResponse(state.lastSpeechText, state.lastSpeechLanguage, false);
    }
  } catch (error) {
    $("responseText").textContent = error.message;
    showToast(error.message, true);
  } finally {
    setCommandBusy(false);
  }
}

function setCommandBusy(busy) {
  $("commandForm").classList.toggle("is-loading", busy);
  $("micButton").disabled = busy && !state.recording;
}

async function startRecording() {
  if (!navigator.mediaDevices?.getUserMedia) {
    showToast("This browser does not support microphone recording. Use Chrome or Edge.", true);
    return;
  }
  if (!state.health?.gnani?.configured) {
    showToast("Add GNANI_API_KEY to .env and restart the server before using voice.", true);
    return;
  }

  try {
    stopPlayback();
    state.stream = await navigator.mediaDevices.getUserMedia({
      audio: {
        channelCount: 1,
        echoCancellation: true,
        noiseSuppression: true,
        autoGainControl: true,
      },
    });

    const AudioContextClass = window.AudioContext || window.webkitAudioContext;
    state.audioContext = new AudioContextClass();
    state.sampleRate = state.audioContext.sampleRate;
    state.sourceNode = state.audioContext.createMediaStreamSource(state.stream);
    state.analyser = state.audioContext.createAnalyser();
    state.analyser.fftSize = 2048;
    state.analyser.smoothingTimeConstant = 0.74;
    state.processor = state.audioContext.createScriptProcessor(4096, 1, 1);
    state.silentGain = state.audioContext.createGain();
    state.silentGain.gain.value = 0;
    state.chunks = [];

    state.sourceNode.connect(state.analyser);
    state.analyser.connect(state.processor);
    state.processor.connect(state.silentGain);
    state.silentGain.connect(state.audioContext.destination);
    state.processor.onaudioprocess = (event) => {
      if (!state.recording) return;
      state.chunks.push(new Float32Array(event.inputBuffer.getChannelData(0)));
    };

    state.recording = true;
    state.recordingStartedAt = Date.now();
    $("micButton").classList.add("is-recording");
    $("micButton").setAttribute("aria-label", "Stop recording");
    $("recordingLabel").textContent = "Listening — speak now";
    $("sttMeta").textContent = "Telugu + English code-mixed speech supported";
    updateTimer();
    state.timerInterval = window.setInterval(updateTimer, 250);
    state.maxRecordingTimer = window.setTimeout(() => stopRecording(true), 25000);
    drawLiveWaveform();
  } catch (error) {
    cleanupRecorder();
    const message = error.name === "NotAllowedError"
      ? "Microphone permission was blocked. Allow microphone access in the browser address bar."
      : `Could not start the microphone: ${error.message}`;
    showToast(message, true);
  }
}

async function stopRecording(upload) {
  if (!state.recording) return;
  state.recording = false;
  window.clearInterval(state.timerInterval);
  window.clearTimeout(state.maxRecordingTimer);
  cancelAnimationFrame(state.animationFrame);

  const chunks = state.chunks.slice();
  const inputRate = state.sampleRate;
  cleanupRecorder();
  resetRecordingUI();
  drawIdleWaveform();

  if (!upload) return;
  const combined = mergeFloat32(chunks);
  if (combined.length < inputRate * 0.25) {
    showToast("The recording was too short. Please speak for at least one second.", true);
    return;
  }

  const downsampled = downsampleBuffer(combined, inputRate, 16000);
  const wav = encodeWAV(downsampled, 16000);
  await uploadAudio(wav);
}

function cleanupRecorder() {
  try { state.processor?.disconnect(); } catch (_) {}
  try { state.analyser?.disconnect(); } catch (_) {}
  try { state.sourceNode?.disconnect(); } catch (_) {}
  try { state.silentGain?.disconnect(); } catch (_) {}
  state.stream?.getTracks().forEach((track) => track.stop());
  if (state.audioContext && state.audioContext.state !== "closed") {
    state.audioContext.close().catch(() => {});
  }
  state.stream = null;
  state.audioContext = null;
  state.sourceNode = null;
  state.analyser = null;
  state.processor = null;
  state.silentGain = null;
  state.chunks = [];
}

function resetRecordingUI() {
  $("micButton").classList.remove("is-recording");
  $("micButton").setAttribute("aria-label", "Start recording");
  $("recordingLabel").textContent = "Processing voice command";
  $("recordingTimer").textContent = "00:00";
}

function updateTimer() {
  const elapsed = Math.max(0, Date.now() - state.recordingStartedAt);
  const seconds = Math.floor(elapsed / 1000);
  $("recordingTimer").textContent = `00:${String(seconds).padStart(2, "0")}`;
}

async function uploadAudio(wavBuffer) {
  $("sttMeta").textContent = "Sending audio securely to Gnani Prisma...";
  $("recordingLabel").textContent = "Transcribing";
  setCommandBusy(true);
  try {
    const form = new FormData();
    form.append("audio", new Blob([wavBuffer], { type: "audio/wav" }), "voice-command.wav");
    const response = await fetch("/api/stt", { method: "POST", body: form });
    if (!response.ok) throw new Error(await responseError(response));
    const data = await response.json();
    const result = data.result;
    const transcript = String(result.transcript || "").trim();
    if (!transcript) throw new Error("No speech was detected. Try again closer to the microphone.");

    const latency = result.endToEndLatency
      ? `${Math.round(result.endToEndLatency * 1000)} ms`
      : "completed";
    const language = result.detectedLanguageCode || "auto detected";
    const fallback = result.fallbackUsed ? " · v2.5 fallback" : "";
    $("sttMeta").textContent = `${result.model || "Prisma"} · ${language} · ${latency}${fallback}`;
    $("recordingLabel").textContent = "Transcription complete";
    $("transcriptText").textContent = transcript;
    await executeCommand(transcript);
  } catch (error) {
    $("recordingLabel").textContent = "Ready to try again";
    $("sttMeta").textContent = error.message;
    showToast(error.message, true);
  } finally {
    setCommandBusy(false);
  }
}

async function speakResponse(text, language, force) {
  if (!text || (!force && !$("autoSpeak").checked)) return;
  stopPlayback();

  if (state.health?.gnani?.configured) {
    try {
      const response = await fetch("/api/tts", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ text, language }),
      });
      if (!response.ok) throw new Error(await responseError(response));
      const blob = await response.blob();
      state.currentAudioURL = URL.createObjectURL(blob);
      const audio = $("responseAudio");
      audio.src = state.currentAudioURL;
      await audio.play();
      return;
    } catch (error) {
      showToast(`Gnani voice unavailable; using browser speech. ${error.message}`, true);
    }
  }
  browserSpeak(text, language);
}

function browserSpeak(text, language) {
  if (!("speechSynthesis" in window)) return;
  window.speechSynthesis.cancel();
  const utterance = new SpeechSynthesisUtterance(text);
  utterance.lang = language === "te-IN" ? "te-IN" : language === "hi-IN" ? "hi-IN" : "en-IN";
  utterance.rate = 0.96;
  window.speechSynthesis.speak(utterance);
}

function stopPlayback() {
  const audio = $("responseAudio");
  audio.pause();
  audio.removeAttribute("src");
  if (state.currentAudioURL) {
    URL.revokeObjectURL(state.currentAudioURL);
    state.currentAudioURL = "";
  }
  if ("speechSynthesis" in window) window.speechSynthesis.cancel();
}

function drawIdleWaveform() {
  const canvas = $("waveform");
  const rect = canvas.getBoundingClientRect();
  const dpr = window.devicePixelRatio || 1;
  canvas.width = Math.max(1, Math.floor(rect.width * dpr));
  canvas.height = Math.max(1, Math.floor(rect.height * dpr));
  const ctx = canvas.getContext("2d");
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
  ctx.clearRect(0, 0, rect.width, rect.height);
  ctx.strokeStyle = "rgba(94, 229, 166, 0.22)";
  ctx.lineWidth = 1.4;
  ctx.beginPath();
  for (let x = 0; x <= rect.width; x += 3) {
    const y = rect.height / 2 + Math.sin(x / 27) * 5 + Math.sin(x / 11) * 2;
    if (x === 0) ctx.moveTo(x, y); else ctx.lineTo(x, y);
  }
  ctx.stroke();
}

function drawLiveWaveform() {
  if (!state.recording || !state.analyser) return;
  const canvas = $("waveform");
  const rect = canvas.getBoundingClientRect();
  const dpr = window.devicePixelRatio || 1;
  if (canvas.width !== Math.floor(rect.width * dpr) || canvas.height !== Math.floor(rect.height * dpr)) {
    canvas.width = Math.max(1, Math.floor(rect.width * dpr));
    canvas.height = Math.max(1, Math.floor(rect.height * dpr));
  }
  const ctx = canvas.getContext("2d");
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
  ctx.clearRect(0, 0, rect.width, rect.height);

  const data = new Uint8Array(state.analyser.fftSize);
  state.analyser.getByteTimeDomainData(data);
  const gradient = ctx.createLinearGradient(0, 0, rect.width, 0);
  gradient.addColorStop(0, "rgba(94,229,166,0.18)");
  gradient.addColorStop(0.5, "rgba(94,229,166,0.95)");
  gradient.addColorStop(1, "rgba(255,180,94,0.28)");
  ctx.strokeStyle = gradient;
  ctx.lineWidth = 2;
  ctx.beginPath();
  const slice = rect.width / data.length;
  let x = 0;
  for (let i = 0; i < data.length; i++) {
    const y = (data[i] / 128) * (rect.height / 2);
    if (i === 0) ctx.moveTo(x, y); else ctx.lineTo(x, y);
    x += slice;
  }
  ctx.stroke();
  state.animationFrame = requestAnimationFrame(drawLiveWaveform);
}

function mergeFloat32(chunks) {
  const length = chunks.reduce((sum, chunk) => sum + chunk.length, 0);
  const result = new Float32Array(length);
  let offset = 0;
  for (const chunk of chunks) {
    result.set(chunk, offset);
    offset += chunk.length;
  }
  return result;
}

function downsampleBuffer(buffer, inputRate, outputRate) {
  if (outputRate === inputRate) return buffer;
  if (outputRate > inputRate) throw new Error("Output sample rate must not exceed input sample rate");
  const ratio = inputRate / outputRate;
  const newLength = Math.round(buffer.length / ratio);
  const result = new Float32Array(newLength);
  let resultIndex = 0;
  let bufferIndex = 0;
  while (resultIndex < result.length) {
    const nextBufferIndex = Math.round((resultIndex + 1) * ratio);
    let accumulator = 0;
    let count = 0;
    for (let i = bufferIndex; i < nextBufferIndex && i < buffer.length; i++) {
      accumulator += buffer[i];
      count++;
    }
    result[resultIndex] = count ? accumulator / count : 0;
    resultIndex++;
    bufferIndex = nextBufferIndex;
  }
  return result;
}

function encodeWAV(samples, sampleRate) {
  const buffer = new ArrayBuffer(44 + samples.length * 2);
  const view = new DataView(buffer);
  writeASCII(view, 0, "RIFF");
  view.setUint32(4, 36 + samples.length * 2, true);
  writeASCII(view, 8, "WAVE");
  writeASCII(view, 12, "fmt ");
  view.setUint32(16, 16, true);
  view.setUint16(20, 1, true);
  view.setUint16(22, 1, true);
  view.setUint32(24, sampleRate, true);
  view.setUint32(28, sampleRate * 2, true);
  view.setUint16(32, 2, true);
  view.setUint16(34, 16, true);
  writeASCII(view, 36, "data");
  view.setUint32(40, samples.length * 2, true);
  let offset = 44;
  for (const sample of samples) {
    const clipped = Math.max(-1, Math.min(1, sample));
    view.setInt16(offset, clipped < 0 ? clipped * 0x8000 : clipped * 0x7fff, true);
    offset += 2;
  }
  return buffer;
}

function writeASCII(view, offset, text) {
  for (let i = 0; i < text.length; i++) view.setUint8(offset + i, text.charCodeAt(i));
}

async function handleInventoryAction(event) {
  const button = event.target.closest("[data-product-id][data-action]");
  if (!button) return;
  button.disabled = true;
  try {
    const data = await requestJSON("/api/stock", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        productId: button.dataset.productId,
        action: button.dataset.action,
        quantity: Number(button.dataset.quantity || 1),
        note: "Dashboard quick action",
      }),
    });
    state.dashboard = data.dashboard;
    renderDashboard();
    showToast(button.dataset.action === "sale" ? "Sale recorded." : "Stock added.");
  } catch (error) {
    showToast(error.message, true);
  } finally {
    button.disabled = false;
  }
}

async function undoLastAction() {
  try {
    const data = await requestJSON("/api/undo", { method: "POST" });
    state.dashboard = data.dashboard;
    renderDashboard();
    showToast(data.message || "Last action undone.");
  } catch (error) {
    showToast(error.message, true);
  }
}

async function resetDemo() {
  if (!window.confirm("Reset inventory and sales to the original synthetic demo data?")) return;
  try {
    const data = await requestJSON("/api/reset", { method: "POST" });
    state.dashboard = data.dashboard;
    renderDashboard();
    $("transcriptText").textContent = "Demo data has been reset.";
    $("responseText").textContent = "Ready for a fresh demonstration.";
    showToast("Demo data reset.");
  } catch (error) {
    showToast(error.message, true);
  }
}

function openProductDialog() {
  const dialog = $("productDialog");
  if (typeof dialog.showModal === "function") dialog.showModal();
  else dialog.setAttribute("open", "");
}

function closeProductDialog() {
  const dialog = $("productDialog");
  if (typeof dialog.close === "function") dialog.close();
  else dialog.removeAttribute("open");
}

async function addProduct(event) {
  event.preventDefault();
  const form = event.currentTarget;
  const values = new FormData(form);
  const payload = {
    name: values.get("name"),
    category: values.get("category"),
    unit: values.get("unit"),
    stock: Number(values.get("stock")),
    lowStockThreshold: Number(values.get("lowStockThreshold")),
    targetStock: Number(values.get("targetStock")),
    costPrice: Number(values.get("costPrice")),
    sellPrice: Number(values.get("sellPrice")),
    aliases: String(values.get("aliases") || "").split(",").map((item) => item.trim()).filter(Boolean),
  };
  try {
    const data = await requestJSON("/api/products", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });
    state.dashboard = data.dashboard;
    renderDashboard();
    form.reset();
    closeProductDialog();
    showToast(`${data.product.name} added to inventory.`);
  } catch (error) {
    showToast(error.message, true);
  }
}

function drawSalesChart(points) {
  const canvas = $("salesChart");
  const rect = canvas.getBoundingClientRect();
  if (!rect.width || !rect.height) return;
  const dpr = window.devicePixelRatio || 1;
  canvas.width = Math.floor(rect.width * dpr);
  canvas.height = Math.floor(rect.height * dpr);
  const ctx = canvas.getContext("2d");
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
  ctx.clearRect(0, 0, rect.width, rect.height);

  const padding = { top: 22, right: 18, bottom: 34, left: 54 };
  const width = rect.width - padding.left - padding.right;
  const height = rect.height - padding.top - padding.bottom;
  const values = points.map((point) => Number(point.revenue || 0));
  const maxValue = Math.max(...values, 100);
  const niceMax = Math.ceil(maxValue / 500) * 500 || 500;

  ctx.font = "11px system-ui";
  ctx.textBaseline = "middle";
  for (let i = 0; i <= 4; i++) {
    const y = padding.top + (height / 4) * i;
    ctx.strokeStyle = "rgba(158,176,193,0.10)";
    ctx.lineWidth = 1;
    ctx.beginPath();
    ctx.moveTo(padding.left, y);
    ctx.lineTo(padding.left + width, y);
    ctx.stroke();
    const labelValue = niceMax - (niceMax / 4) * i;
    ctx.fillStyle = "rgba(158,176,193,0.65)";
    ctx.textAlign = "right";
    ctx.fillText(compactCurrency(labelValue), padding.left - 9, y);
  }

  if (!points.length) return;
  const step = width / points.length;
  const barWidth = Math.min(38, step * 0.46);
  const coordinates = [];

  points.forEach((point, index) => {
    const value = Number(point.revenue || 0);
    const barHeight = (value / niceMax) * height;
    const x = padding.left + step * index + step / 2;
    const y = padding.top + height - barHeight;
    const gradient = ctx.createLinearGradient(0, y, 0, padding.top + height);
    gradient.addColorStop(0, "rgba(94,229,166,0.78)");
    gradient.addColorStop(1, "rgba(43,191,125,0.10)");
    ctx.fillStyle = gradient;
    roundedRect(ctx, x - barWidth / 2, y, barWidth, Math.max(3, barHeight), 7);
    ctx.fill();
    coordinates.push([x, y]);

    const date = new Date(`${point.date}T00:00:00`);
    ctx.fillStyle = "rgba(158,176,193,0.68)";
    ctx.textAlign = "center";
    ctx.textBaseline = "top";
    ctx.fillText(date.toLocaleDateString("en-IN", { weekday: "short" }), x, padding.top + height + 10);
  });

  ctx.strokeStyle = "rgba(255,180,94,0.86)";
  ctx.lineWidth = 2;
  ctx.beginPath();
  coordinates.forEach(([x, y], index) => {
    const lineY = y - 7;
    if (index === 0) ctx.moveTo(x, lineY); else ctx.lineTo(x, lineY);
  });
  ctx.stroke();
  coordinates.forEach(([x, y]) => {
    ctx.fillStyle = "#ffb45e";
    ctx.beginPath();
    ctx.arc(x, y - 7, 3.4, 0, Math.PI * 2);
    ctx.fill();
  });
}

function roundedRect(ctx, x, y, width, height, radius) {
  const r = Math.min(radius, width / 2, height / 2);
  ctx.beginPath();
  ctx.moveTo(x + r, y);
  ctx.arcTo(x + width, y, x + width, y + height, r);
  ctx.arcTo(x + width, y + height, x, y + height, r);
  ctx.arcTo(x, y + height, x, y, r);
  ctx.arcTo(x, y, x + width, y, r);
  ctx.closePath();
}

async function requestJSON(url, options = {}) {
  const response = await fetch(url, options);
  if (!response.ok) throw new Error(await responseError(response));
  return response.json();
}

async function responseError(response) {
  const raw = await response.text();
  try {
    const parsed = JSON.parse(raw);
    const base = parsed.error?.message || parsed.message || `Request failed (${response.status})`;
    const detail = parsed.error?.details;
    if (detail && typeof detail === "string" && detail !== base) return `${base}: ${detail}`;
    return base;
  } catch (_) {
    return raw.trim() || `Request failed (${response.status})`;
  }
}

function compactCurrency(value) {
  if (value >= 100000) return `₹${number.format(value / 100000)}L`;
  if (value >= 1000) return `₹${number.format(value / 1000)}K`;
  return `₹${number.format(value)}`;
}

function relativeTime(value) {
  const timestamp = new Date(value).getTime();
  if (!Number.isFinite(timestamp)) return "";
  const diffSeconds = Math.round((timestamp - Date.now()) / 1000);
  const formatter = new Intl.RelativeTimeFormat("en", { numeric: "auto" });
  const abs = Math.abs(diffSeconds);
  if (abs < 60) return formatter.format(diffSeconds, "second");
  if (abs < 3600) return formatter.format(Math.round(diffSeconds / 60), "minute");
  if (abs < 86400) return formatter.format(Math.round(diffSeconds / 3600), "hour");
  return formatter.format(Math.round(diffSeconds / 86400), "day");
}

function escapeHTML(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}

function showToast(message, isError = false) {
  const toast = $("toast");
  toast.textContent = message;
  toast.classList.toggle("error", isError);
  toast.classList.add("show");
  window.clearTimeout(state.toastTimer);
  state.toastTimer = window.setTimeout(() => toast.classList.remove("show"), 4200);
}

function debounce(fn, wait) {
  let timeout = 0;
  return (...args) => {
    window.clearTimeout(timeout);
    timeout = window.setTimeout(() => fn(...args), wait);
  };
}

async function installApp() {
  if (!state.installPrompt) return;
  state.installPrompt.prompt();
  await state.installPrompt.userChoice;
  state.installPrompt = null;
  $("installButton").classList.add("hidden");
}

function registerServiceWorker() {
  if ("serviceWorker" in navigator) {
    navigator.serviceWorker.register("/service-worker.js").catch(() => {});
  }
  renderSuggestions(defaultSuggestions($("responseLanguage").value));
}
