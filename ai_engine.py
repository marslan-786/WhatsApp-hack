import os
import uvicorn
import subprocess
from fastapi import FastAPI, UploadFile, File, Form, Response
from fastapi.responses import FileResponse
from faster_whisper import WhisperModel
from gtts import gTTS # ✅ Google TTS
import torch

app = FastAPI()

TEMP_DIR = "/app/temp_ai"
os.makedirs(TEMP_DIR, exist_ok=True)

print("⏳ [PYTHON] Loading Whisper (Ears)...")
stt_model = WhisperModel("large-v3", device="cpu", compute_type="int8")

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
    rand_id = os.urandom(4).hex()
    raw_mp3_path = os.path.join(TEMP_DIR, f"raw_{rand_id}.mp3")
    final_ogg_path = os.path.join(TEMP_DIR, f"out_{rand_id}.opus")
    
    try:
        # 🔥 STEP 1: Google TTS Generation
        # Ye Google ke servers use karega, jo 99.9% reliable hain
        tts = gTTS(text=text, lang='ur', slow=False)
        tts.save(raw_mp3_path)

        if not os.path.exists(raw_mp3_path) or os.path.getsize(raw_mp3_path) == 0:
            return Response(content="gTTS generated empty file", status_code=500)

        # 🔥 STEP 2: Convert to WhatsApp OGG
        cmd_ffmpeg = [
            "ffmpeg", "-y",
            "-i", raw_mp3_path,
            "-vn", "-c:a", "libopus", "-b:a", "24k", "-ac", "1", "-f", "ogg", 
            final_ogg_path
        ]
        subprocess.run(cmd_ffmpeg, check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)

    except Exception as e:
        print(f"❌ gTTS Error: {e}")
        return Response(content=str(e), status_code=500)
    
    if os.path.exists(raw_mp3_path): os.remove(raw_mp3_path)

    return FileResponse(final_ogg_path, media_type="audio/ogg")

if __name__ == "__main__":
    uvicorn.run(app, host="0.0.0.0", port=5000)
