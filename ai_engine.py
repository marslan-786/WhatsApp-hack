import os
import uvicorn
import subprocess
from fastapi import FastAPI, UploadFile, File, Form
from fastapi.responses import FileResponse
from faster_whisper import WhisperModel
import torch

app = FastAPI()

# Setup Paths
TEMP_DIR = "/app/temp_ai"
# ✅ یہ پاتھ Dockerfile کے ساتھ میچ کر رہا ہے
MODEL_PATH = "/app/models/ur_pk.onnx" 
PIPER_BIN = "/usr/local/bin/piper/piper"

os.makedirs(TEMP_DIR, exist_ok=True)

# Load Whisper
print("⏳ [PYTHON] Loading Whisper (Ears)...")
stt_model = WhisperModel("large-v3", device="cpu", compute_type="int8")

@app.post("/transcribe")
async def transcribe(file: UploadFile = File(...)):
    file_path = os.path.join(TEMP_DIR, file.filename)
    with open(file_path, "wb") as buffer:
        buffer.write(await file.read())
    
    # CPU cores are plenty, so beam_size 5 is safe
    segments, info = stt_model.transcribe(file_path, beam_size=5)
    text = "".join([segment.text for segment in segments])
    
    os.remove(file_path)
    return {"text": text, "language": info.language}

@app.post("/speak")
async def speak(text: str = Form(...), lang: str = Form("ur")):
    rand_id = os.urandom(4).hex()
    raw_wav_path = os.path.join(TEMP_DIR, f"raw_{rand_id}.wav")
    final_ogg_path = os.path.join(TEMP_DIR, f"out_{rand_id}.opus")
    
    try:
        # 🔥 STEP 1: Piper TTS Generation
        # -s 0 ka matlab hai "Speaker 0" (Urdu single speaker)
        cmd_piper = f'echo "{text}" | {PIPER_BIN} --model {MODEL_PATH} --output_file {raw_wav_path}'
        
        # Run Piper
        result = subprocess.run(cmd_piper, shell=True, capture_output=True, text=True)
        
        if result.returncode != 0:
            print(f"❌ Piper Failed: {result.stderr}")
            return {"error": f"Piper Error: {result.stderr}"}

        # Check File
        if not os.path.exists(raw_wav_path) or os.path.getsize(raw_wav_path) == 0:
            return {"error": "Piper generated empty file"}

        # 🔥 STEP 2: Convert to OGG
        cmd_ffmpeg = [
            "ffmpeg", "-y",
            "-i", raw_wav_path,
            "-vn", "-c:a", "libopus", "-b:a", "24k", "-ac", "1", "-f", "ogg", 
            final_ogg_path
        ]
        subprocess.run(cmd_ffmpeg, check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)

    except Exception as e:
        print(f"❌ Critical Error: {e}")
        return {"error": str(e)}
    
    # Cleanup
    if os.path.exists(raw_wav_path): os.remove(raw_wav_path)

    return FileResponse(final_ogg_path, media_type="audio/ogg")

if __name__ == "__main__":
    uvicorn.run(app, host="0.0.0.0", port=5000)
