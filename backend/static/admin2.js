// --- PROJEKT-BEARBEITUNG & VORSCHAU-LOGIK ---
function editProject(id, folder, title, date, desc, buttonElement) {
  document.getElementById('formTitle').textContent = "Projeyi Düzenle (ID: " + id + ")";
  document.getElementById('editProjectId').value = id;
  document.getElementById('formFolder').value = folder;
  document.getElementById('formTitleInput').value = title;
  document.getElementById('formDate').value = date;
  document.getElementById('formDesc').value = desc;

  document.getElementById('formMainImage').required = false;
  document.getElementById('formGalleryMedia').required = false;

  document.getElementById('mainProjectForm').action = "/admin/update";
  document.getElementById('formSubmitBtn').textContent = "Değişiklikleri Kaydet";
  document.getElementById('formCancelBtn').style.display = "block";

  // --- 1. HAUPTBILD VORSCHAU ---
  const mainPreview = document.getElementById('editMainPreview');
  mainPreview.innerHTML = '';
  const mainImgPath = buttonElement.getAttribute('data-main') || "";

  if (mainImgPath.trim() !== "" && mainImgPath !== "null") {
    mainPreview.style.display = "flex";
    
    const box = document.createElement('div');
    box.className = 'preview-box';
    box.innerHTML = `<img src="${mainImgPath}" alt="Ana Görsel" style="width:100%; height:100%; object-fit:cover; display:block;">`;

    const delBtn = document.createElement('button');
    delBtn.type = 'button';
    delBtn.className = 'btn-mini-delete';
    delBtn.innerHTML = '&times;';
    delBtn.onclick = async function() {
      if(confirm("Ana görseli silmek istiyor musunuz? (Kartın düzgün görünmesi için yeni bir görsel yüklemeniz gerekecektir.)")) {
        const formData = new FormData();
        formData.append("id", id);
        formData.append("path", mainImgPath);
        formData.append("type", "main");
        
        const res = await fetch("/admin/delete-media", { method: "POST", body: formData });
        if(res.ok) {
          box.remove();
          mainPreview.style.display = "none";
          buttonElement.setAttribute('data-main', '');
          document.getElementById('formMainImage').required = true;
        }
      }
    };
    box.appendChild(delBtn);
    mainPreview.appendChild(box);
  } else {
    mainPreview.style.display = "none";
    document.getElementById('formMainImage').required = true;
  }

  // --- 2. GALERIE VORSCHAU ---
  const galleryPreview = document.getElementById('editMediaPreview');
  galleryPreview.innerHTML = '';
  const galleryStr = buttonElement.getAttribute('data-gallery') || "";
  
  if (galleryStr.trim() !== "" && galleryStr !== "null") {
    galleryPreview.style.display = "flex";
    const paths = galleryStr.split(',');
    
    paths.forEach(path => {
      if(!path) return;
      
      const box = document.createElement('div');
      box.className = 'preview-box';
      
      const isVideo = path.toLowerCase().endsWith('.mp4') || path.toLowerCase().endsWith('.webm') || path.toLowerCase().endsWith('.mov');
      if (isVideo) {
        box.innerHTML = `<video src="${path}" muted preload="metadata" style="width:100%; height:100%; object-fit:cover; display:block;"></video>`;
      } else {
        box.innerHTML = `<img src="${path}" alt="Önizleme" style="width:100%; height:100%; object-fit:cover; display:block;">`;
      }
      
      const delBtn = document.createElement('button');
      delBtn.type = 'button';
      delBtn.className = 'btn-mini-delete';
      delBtn.innerHTML = '&times;';
      delBtn.onclick = async function() {
        if(confirm("Bu resmi galeriden silmek istiyor musunuz?")) {
          const formData = new FormData();
          formData.append("id", id);
          formData.append("path", path);
          formData.append("type", "gallery");
          
          const res = await fetch("/admin/delete-media", { method: "POST", body: formData });
          if(res.ok) {
            box.remove();
            // KORRIGIERT: Sicherer Re-Filter des Attribut-Strings bei dynamischen Löschvorgängen
            const currentGallery = buttonElement.getAttribute('data-gallery') || "";
            const updatedGallery = currentGallery.split(',').filter(p => p !== path && p.trim() !== "").join(',');
            buttonElement.setAttribute('data-gallery', updatedGallery);
            if (updatedGallery === "") {
              galleryPreview.style.display = "none";
            }
          }
        }
      };
      
      box.appendChild(delBtn);
      galleryPreview.appendChild(box);
    });
  } else {
    galleryPreview.style.display = "none";
  }

  window.scrollTo({ top: 0, behavior: 'smooth' });
}

function resetFormMode() {
  document.getElementById('formTitle').textContent = "Yeni Proje Ekle";
  document.getElementById('editProjectId').value = "";
  document.getElementById('mainProjectForm').reset();

  document.getElementById('formMainImage').required = true;
  document.getElementById('formGalleryMedia').required = true;

  document.getElementById('mainProjectForm').action = "/admin";
  document.getElementById('formSubmitBtn').textContent = "Projeyi Kaydet ve Yayınla";
  document.getElementById('formCancelBtn').style.display = "none";
  
  document.getElementById('editMainPreview').style.display = "none";
  document.getElementById('editMainPreview').innerHTML = "";
  document.getElementById('editMediaPreview').style.display = "none";
  document.getElementById('editMediaPreview').innerHTML = "";
}

// --- PROMPT EDITOR MODAL LOGIK ---
async function openPromptEditor() {
  try {
    const response = await fetch('/admin/prompt/get');
    if (response.ok) {
      const data = await response.json();
      document.getElementById('promptTextarea').value = data.content;
      document.getElementById('promptModal').style.display = 'flex';
    } else {
      alert('Prompt verileri yüklenemedi!');
    }
  } catch (error) {
    console.error('Prompt hatası:', error);
  }
}

function closePromptEditor() {
  document.getElementById('promptModal').style.display = 'none';
  document.getElementById('promptTextarea').value = '';
}

async function savePromptEditor() {
  const textarea = document.getElementById('promptTextarea');
  const textContent = textarea.value;

  try {
    const response = await fetch('/admin/prompt/save', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ content: textContent })
    });

    if (response.ok) {
      alert('Firmendaten (RAG) başarıyla güncellendi! Yapay zeka yeni bilgileri hemen kullanmaya başlayacaktır.');
      closePromptEditor();
    } else {
      alert('Kayıt işlemi başarısız oldu!');
    }
  } catch (error) {
    console.error('Prompt kaydetme hatası:', error);
    alert('Bağlantı hatası oluştu!');
  }
}
