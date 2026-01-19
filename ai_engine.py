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
os.makedirs(TEMP_DIR, exist_ok=True)

# Load Whisper
print("⏳ [PYTHON] Loading Whisper (Ears)...")
stt_model = WhisperModel("large-v3", device="cuda" if torch.cuda.is_available() else "cpu", compute_type="float16" if torch.cuda.is_available() else "int8")

# Voice Config
VOICE_NAME = "ur-PK-SalmanNeural"

@app.post("/transcribe")
async def transcribe(file: UploadFile = File(...)):
    file_path = os.path.join(TEMP_DIR, file.filename)
    with open(file_path, "wb") as buffer:
        buffer.write(await file.read())
    
    segments, info = stt_model.transcribe(file_path, beam_size=5)
    text = "".join([segment.text for segment in segments])
    
    os.remove(file_path)
    return {"text": text, "language": info.language}

@app.post("/speak")
async def speak(text: str = Form(...), lang: str = Form("ur")):
    # Random filenames
    rand_id = os.urandom(4).hex()
    raw_mp3_path = os.path.join(TEMP_DIR, f"raw_{rand_id}.mp3")
    final_ogg_path = os.path.join(TEMP_DIR, f"out_{rand_id}.opus")
    
    try:
        # 🟢 STEP 1: Generate Audio using Edge-TTS CLI (Most Reliable Method)
        # یہ کمانڈ لائن کے ذریعے چلے گا جو کہ زیادہ مستحکم ہے
        cmd_tts = [
            "edge-tts",
            "--voice", VOICE_NAME,
            "--text", text,
            "--write-media", raw_mp3_path
        ]
        
        # Run command and capture output for debugging
        result = subprocess.run(cmd_tts, capture_output=True, text=True)
        
        if result.returncode != 0:
            print(f"❌ Edge-TTS CLI Error: {result.stderr}")
            return {"error": f"TTS Failed: {result.stderr}"}

        # Check if file exists and has size
        if not os.path.exists(raw_mp3_path) or os.path.getsize(raw_mp3_path) == 0:
            print("❌ Error: Generated MP3 is empty or missing.")
            return {"error": "Empty audio file generated"}

        # 🟢 STEP 2: Convert to WhatsApp OGG/Opus
        cmd_ffmpeg = [
            "ffmpeg", "-y",
            "-i", raw_mp3_path,
            "-vn", 
            "-c:a", "libopus", 
            "-b:a", "24k",  # Thora behtar bitrate
            "-ac", "1", 
            "-f", "ogg", 
            final_ogg_path
        ]
        
        subprocess.run(cmd_ffmpeg, check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)

    except Exception as e:
        print(f"❌ Critical Exception: {e}")
        return {"error": str(e)}
    
    # Cleanup MP3
    if os.path.exists(raw_mp3_path): os.remove(raw_mp3_path)

    # Final Check
    if not os.path.exists(final_ogg_path):
        return {"error": "Final conversion failed"}

    return FileResponse(final_ogg_path, media_type="audio/ogg")

if __name__ == "__main__":
    uvicorn.run(app, host="0.0.0.0", port=5000)
