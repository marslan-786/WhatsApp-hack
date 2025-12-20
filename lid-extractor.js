const { Client } = require('pg');
const fs = require('fs');

async function extractSelfLid() {
    console.log("\n" + "═".repeat(60));
    console.log("🛡️ [SECURE LID SYSTEM] بوٹ کی اپنی آئی ڈی تلاش کی جا رہی ہے...");
    console.log("═".repeat(60));

    const client = new Client({
        connectionString: process.env.DATABASE_URL,
        ssl: { rejectUnauthorized: false }
    });

    try {
        await client.connect();
        console.log("✅ [DATABASE] پوسٹ گریس کے ساتھ لنک ہو گیا ہے۔");

        // 1. وہ جے آئی ڈیز نکالیں جن سے بوٹ لاگ ان ہے
        const deviceRes = await client.query('SELECT jid FROM whatsmeow_device;');
        
        let botData = {};

        for (let row of deviceRes.rows) {
            const botFullJid = row.jid; // مثال: 92301...@s.whatsapp.net
            const pureNumber = botFullJid.split('@')[0].split(':')[0];

            console.log(`\n🔍 [CHECKING BOT] فون نمبر: ${pureNumber}`);

            // 2. اس نمبر کا Push Name تلاش کریں (صحیح کالم 'their_jid' استعمال کرتے ہوئے)
            const nameQuery = `SELECT push_name FROM whatsmeow_contacts WHERE their_jid = $1 LIMIT 1;`;
            const nameRes = await client.query(nameQuery, [botFullJid]);
            
            let botName = nameRes.rows[0]?.push_name;

            if (botName) {
                console.log(`👤 [PROFILE NAME] بوٹ کا نام ملا: "${botName}"`);
                
                // نام کے ذریعے LID تلاش کریں
                const lidQuery = `
                    SELECT their_jid FROM whatsmeow_contacts 
                    WHERE push_name = $1 
                    AND their_jid LIKE '%@lid' 
                    LIMIT 1;
                `;
                const lidRes = await client.query(lidQuery, [botName]);

                if (lidRes.rows.length > 0) {
                    const realLid = lidRes.rows[0].their_jid;
                    console.log(`✅ [MATCH FOUND] نام کے ذریعے LID مل گئی: ${realLid}`);
                    botData[pureNumber] = { phone: pureNumber, lid: realLid, method: 'name_match' };
                    continue;
                }
            }

            // 3. اگر نام سے کام نہ بنے، تو نمبر کے پہلے حصے (Prefix) سے تلاش کریں
            console.log(`⏳ [FALLBACK] نام سے LID نہیں ملی، اب نمبر سے سرچ کر رہے ہیں...`);
            const prefixMatch = `${pureNumber.substring(0, 8)}%@lid`; // پہلے 8 ہندسے
            const prefixQuery = `SELECT their_jid FROM whatsmeow_contacts WHERE their_jid LIKE $1 LIMIT 1;`;
            const prefixRes = await client.query(prefixQuery, [prefixMatch]);

            if (prefixRes.rows.length > 0) {
                const realLid = prefixRes.rows[0].their_jid;
                console.log(`✅ [MATCH FOUND] نمبر کے ذریعے LID مل گئی: ${realLid}`);
                botData[pureNumber] = { phone: pureNumber, lid: realLid, method: 'prefix_match' };
            } else {
                console.log(`❌ [FAILED] اس نمبر کی LID ابھی ڈیٹا بیس میں موجود نہیں ہے۔`);
            }
        }

        // 4. فائنل ڈیٹا سیو کریں
        if (Object.keys(botData).length > 0) {
            fs.writeFileSync('./lid_data.json', JSON.stringify({ bots: botData }, null, 2));
            console.log("\n💾 [SUCCESS] ڈیٹا 'lid_data.json' میں محفوظ ہو گیا ہے۔");
        }

    } catch (err) {
        console.error("\n❌ [ERROR]:", err.message);
    } finally {
        await client.end();
        console.log("🏁 [FINISHED]");
        console.log("═".repeat(60) + "\n");
        process.exit(0);
    }
}

extractSelfLid();