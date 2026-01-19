# ai_engine.py
import os
import uvicorn
from fastapi import FastAPI, UploadFile, File, Form
from fastapi.responses import FileResponse, JSONResponse
from faster_whisper import WhisperModel
from TTS.api import TTS
import torch

app = FastAPI()

# 1. SETUP PATHS
TEMP_DIR = "/app/temp_ai"
os.makedirs(TEMP_DIR, exist_ok=True)

# 2. LOAD MODELS (Start hone par load honge)
print("⏳ [PYTHON] Loading Whisper (Ears)...")
# 'large-v3' heavy hai, agar slow ho to 'medium' kar lena
stt_model = WhisperModel("large-v3", device="cuda" if torch.cuda.is_available() else "cpu", compute_type="float16" if torch.cuda.is_available() else "int8")

print("⏳ [PYTHON] Loading XTTS (Voice)...")
# GPU use karega agar available hua
tts_engine = TTS("tts_models/multilingual/multi-dataset/xtts_v2").to("cuda" if torch.cuda.is_available() else "cpu")

@app.get("/health")
def health_check():
    return {"status": "running", "gpu": torch.cuda.is_available()}

@app.post("/transcribe")
async def transcribe(file: UploadFile = File(...)):
    """Audio sun kar Text wapis karega"""
    file_path = os.path.join(TEMP_DIR, file.filename)
    
    with open(file_path, "wb") as buffer:
        buffer.write(await file.read())
    
    # Transcribe logic
    segments, info = stt_model.transcribe(file_path, beam_size=5)
    text = "".join([segment.text for segment in segments])
    
    os.remove(file_path) # Cleanup
    return {"text": text, "language": info.language}

@app.post("/speak")
async def speak(text: str = Form(...), speaker_wav: UploadFile = File(...), lang: str = Form("hi")): # ✅ Default changed to 'hi'
    """Text aur Reference Audio le kar Voice Note banaye ga"""
    
    # 🛑 Language Override (Safety Check)
    # اگر غلطی سے 'ur' آ بھی جائے تو اسے 'hi' کر دو ورنہ کریش ہوگا
    if lang == "ur":
        lang = "hi"

    ref_path = os.path.join(TEMP_DIR, "ref_" + speaker_wav.filename)
    out_path = os.path.join(TEMP_DIR, f"out_{os.urandom(4).hex()}.wav")
    
    # Save Reference Audio temporarily
    with open(ref_path, "wb") as buffer:
        buffer.write(await speaker_wav.read())
        
    # Generate Voice
    try:
        tts_engine.tts_to_file(
            text=text,
            file_path=out_path,
            speaker_wav=ref_path,
            language=lang 
        )
    except Exception as e:
        print(f"❌ TTS Error: {e}")
        return {"error": str(e)}
    
    os.remove(ref_path) # Cleanup ref
    return FileResponse(out_path, media_type="audio/wav")


if __name__ == "__main__":
    # Internal port 5000 par chalega
    uvicorn.run(app, host="0.0.0.0", port=5000)
