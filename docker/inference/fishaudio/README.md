# FishAudio TTS API 文档

FishAudio 是一个强大的文本转语音（TTS）服务，支持多种声音模型和丰富的情绪表达。

## API 端点

### POST /v1/tts

将文本转换为语音的 API 端点。

#### 请求头

```
Content-Type: application/json
```

#### 请求体参数

| 参数名 | 类型 | 必填 | 默认值 | 说明 |
|--------|------|------|--------|------|
| `text` | string | 是 | - | 要转换为语音的文本 |
| `temperature` | number | 否 | 0.9 | 控制语音生成的随机性。较高的值（如 1.0）使输出更随机，较低的值（如 0.1）使输出更确定性。建议 s1 模型使用 0.9<br>**范围**: 0 <= x <= 1 |
| `top_p` | number | 否 | 0.9 | 通过核采样控制多样性。较低的值（如 0.1）使输出更聚焦，较高的值（如 1.0）允许更多多样性。建议 s1 模型使用 0.9<br>**范围**: 0 <= x <= 1 |
| `references` | ReferenceAudio[] \| null | 否 | null | 用于语音的参考音频，需要 MessagePack 序列化，这将覆盖 `reference_voices` 和 `reference_texts` |
| `reference_id` | string \| null | 否 | null | 用于语音的参考模型 ID |
| `prosody` | ProsodyControl | 否 | - | 用于语音的韵律控制 |
| `prosody.speed` | number | 否 | 1 | 语速控制 |
| `prosody.volume` | number | 否 | 0 | 音量控制 |
| `chunk_length` | integer | 否 | 200 | 用于语音的块长度<br>**范围**: 100 <= x <= 300 |
| `normalize` | boolean | 否 | true | 是否规范化语音，这将减少延迟但可能降低数字和日期的性能 |
| `format` | enum<string> | 否 | mp3 | 用于语音的格式<br>**可选值**: wav, pcm, mp3, opus |
| `sample_rate` | integer \| null | 否 | null | 用于语音的采样率 |
| `mp3_bitrate` | enum<integer> | 否 | 128 | MP3 比特率<br>**可选值**: 64, 128, 192 |
| `opus_bitrate` | enum<integer> | 否 | 32 | Opus 比特率<br>**可选值**: -1000, 24, 32, 48, 64 |
| `latency` | enum<string> | 否 | normal | 用于语音的延迟模式，balanced 将减少延迟但可能导致性能下降<br>**可选值**: normal, balanced |

#### 请求示例

```json
{
  "text": "你好，这是一段测试文本。",
  "temperature": 0.9,
  "top_p": 0.9,
  "reference_id": "xiaowan",
  "prosody": {
    "speed": 1.0,
    "volume": 0
  },
  "chunk_length": 200,
  "normalize": true,
  "format": "mp3",
  "mp3_bitrate": 128,
  "latency": "normal"
}
```

#### 响应

成功响应将返回音频文件的二进制数据，Content-Type 根据请求的 `format` 参数设置（如 `audio/mpeg` 对于 mp3）。

## 情绪标签使用指南

在文本中使用情绪标签可以控制语音的情感表达。标签格式为 `(标签名)`。

### 基础情绪（24 种）

| 情绪 | 标签 | 描述 | 使用场景示例 |
|------|------|------|------------|
| Happy | `(happy)` | 欢快、乐观的语调 | 好消息、问候 |
| Sad | `(sad)` | 忧郁、沮丧的语调 | 同情、坏消息 |
| Angry | `(angry)` | 愤怒、攻击性的语调 | 抱怨、警告 |
| Excited | `(excited)` | 充满活力、热情的语调 | 公告、庆祝 |
| Calm | `(calm)` | 平静、放松的语调 | 说明、冥想 |
| Nervous | `(nervous)` | 焦虑、不确定的语调 | 免责声明、道歉 |
| Confident | `(confident)` | 自信、坚定的语调 | 演示、销售 |
| Surprised | `(surprised)` | 震惊、惊讶的语调 | 反应、发现 |
| Satisfied | `(satisfied)` | 满足、满意的语调 | 确认、评价 |
| Delighted | `(delighted)` | 非常高兴、愉悦的语调 | 庆祝、赞美 |
| Scared | `(scared)` | 害怕、恐惧的语调 | 警告、恐怖故事 |
| Worried | `(worried)` | 担忧、困扰的语调 | 担忧、问题 |
| Upset | `(upset)` | 不安、痛苦的语调 | 抱怨、问题 |
| Frustrated | `(frustrated)` | 恼怒、沮丧的语调 | 技术问题、延误 |
| Depressed | `(depressed)` | 非常悲伤、绝望的语调 | 严肃话题 |
| Empathetic | `(empathetic)` | 理解、关怀的语调 | 支持、咨询 |
| Embarrassed | `(embarrassed)` | 羞愧、尴尬的语调 | 道歉、错误 |
| Disgusted | `(disgusted)` | 厌恶、反感的语调 | 负面评价 |
| Moved | `(moved)` | 感动的语调 | 感人的时刻 |
| Proud | `(proud)` | 自豪、满意的语调 | 成就、赞美 |
| Relaxed | `(relaxed)` | 轻松、随意的语调 | 随意对话 |
| Grateful | `(grateful)` | 感激、感谢的语调 | 感谢、赞赏 |
| Curious | `(curious)` | 好奇、感兴趣的语调 | 问题、探索 |
| Sarcastic | `(sarcastic)` | 讽刺、嘲弄的语调 | 幽默、批评 |

### 高级情绪（25 种）

| 情绪 | 标签 | 描述 | 使用场景示例 |
|------|------|------|------------|
| Disdainful | `(disdainful)` | 轻蔑、蔑视的语调 | 批评、拒绝 |
| Unhappy | `(unhappy)` | 不满、不满意的语调 | 抱怨、反馈 |
| Anxious | `(anxious)` | 非常担忧、不安的语调 | 紧急事务 |
| Hysterical | `(hysterical)` | 情绪失控的语调 | 极端反应 |
| Indifferent | `(indifferent)` | 漠不关心、中性的语调 | 中性回应 |
| Uncertain | `(uncertain)` | 怀疑、不确定的语调 | 推测、问题 |
| Doubtful | `(doubtful)` | 怀疑、质疑的语调 | 不相信、质疑 |
| Confused | `(confused)` | 困惑、不解的语调 | 澄清请求 |
| Disappointed | `(disappointed)` | 失望、不满意的语调 | 未达预期 |
| Regretful | `(regretful)` | 抱歉、悔恨的语调 | 道歉、错误 |
| Guilty | `(guilty)` | 有罪、负责任的语调 | 忏悔、道歉 |
| Ashamed | `(ashamed)` | 深深羞愧的语调 | 严重错误 |
| Jealous | `(jealous)` | 嫉妒、怨恨的语调 | 比较 |
| Envious | `(envious)` | 羡慕、渴望的语调 | 带有渴望的钦佩 |
| Hopeful | `(hopeful)` | 对未来乐观的语调 | 未来计划 |
| Optimistic | `(optimistic)` | 积极乐观的语调 | 鼓励 |
| Pessimistic | `(pessimistic)` | 消极悲观的语调 | 警告、怀疑 |
| Nostalgic | `(nostalgic)` | 怀念过去的语调 | 回忆、故事 |
| Lonely | `(lonely)` | 孤独、孤立的语调 | 情感内容 |
| Bored | `(bored)` | 不感兴趣、厌倦的语调 | 不感兴趣 |
| Contemptuous | `(contemptuous)` | 显示轻蔑的语调 | 强烈批评 |
| Sympathetic | `(sympathetic)` | 显示同情的语调 | 慰问 |
| Compassionate | `(compassionate)` | 显示深切关怀的语调 | 支持、帮助 |
| Determined | `(determined)` | 坚决、决定的语调 | 目标、承诺 |
| Resigned | `(resigned)` | 接受失败的语调 | 放弃、接受 |

### 音调标记（5 种）

控制音量和强度：

| 音调 | 标签 | 描述 | 使用场景 |
|------|------|------|---------|
| Hurried | `(in a hurry tone)` | 匆忙、紧急的语调 | 时间敏感信息 |
| Shouting | `(shouting)` | 大声、呼喊的语调 | 引起注意 |
| Screaming | `(screaming)` | 非常大声、恐慌的语调 | 紧急情况、恐惧 |
| Whispering | `(whispering)` | 非常轻柔、秘密的语调 | 秘密、安静场景 |
| Soft | `(soft tone)` | 温和、安静的语调 | 安慰、摇篮曲 |

### 音频效果（10 种）

添加自然的人声效果：

| 效果 | 标签 | 描述 | 建议文本 |
|------|------|------|---------|
| Laughing | `(laughing)` | 大笑 | Ha, ha, ha |
| Chuckling | `(chuckling)` | 轻笑 | Heh, heh |
| Sobbing | `(sobbing)` | 抽泣 | （可选） |
| Crying Loudly | `(crying loudly)` | 大声哭泣 | （可选） |
| Sighing | `(sighing)` | 叹息 | sigh |
| Groaning | `(groaning)` | 呻吟声 | ugh |
| Panting | `(panting)` | 喘气声 | huff, puff |
| Gasping | `(gasping)` | 倒吸一口气 | gasp |
| Yawning | `(yawning)` | 打哈欠 | yawn |
| Snoring | `(snoring)` | 打鼾声 | zzz |

### 特殊效果

额外的标记用于氛围和上下文：

| 效果 | 标签 | 描述 |
|------|------|------|
| Audience Laughter | `(audience laughing)` | 观众笑声 |
| Background Laughter | `(background laughter)` | 背景笑声 |
| Crowd Laughter | `(crowd laughing)` | 人群笑声 |
| Short Pause | `(break)` | 短暂停顿 |
| Long Pause | `(long-break)` | 长时间停顿 |

**注意**：你也可以使用自然表达，如 "Ha,ha,ha" 来表示笑声，无需使用标签。

### 情绪标签使用示例

```
文本: "今天天气真好！(happy) 我们去公园玩吧。(excited)"
```

```
文本: "对不起，我迟到了。(nervous) 路上堵车了。(upset)"
```

```
文本: "这个产品太棒了！(delighted) 我强烈推荐给大家。(confident)"
```

## 声音模型列表
声音文件 repo: https://opencsg.com/datasets/James/voices?tab=summary

以下是可用的声音模型列表，每个模型都有独特的特征和适用场景。

| 标题 | ID | 特征描述 |
|------|-----|---------|
| adrian | adrian | 外国男音 |
| e-girl | e-girl | 外国女音 |
| 马斯克 | musk | 外国男音 |
| 川普 | trump | 外国男音 |
| horror | horror | 游戏配音 |
| 相声声音 | wangkunshengyin | 男性，中等体型，专业语气 |
| 蒋委员长 | jiangjieshi | 男性，年长，权威且正式 |
| 郑翔洲 | zhengxiangzhou | 男性，成熟，自信的商业声音 |
| AD学姐 | xuejie | 女性，苗条，活泼且年轻 |
| 小明剑魔 | xiaomingjianmo | 男性，年轻游戏玩家，沮丧且咆哮 |
| 央视配音 | zhongyangpeiyin | 男性，深沉共鸣，纪录片风格 |
| 女大学生 | nv daxuesheng | 女性，娇小，疲惫却乐观的学生 |
| 赛马娘 | saimanian | 女性，运动型，激励且活力四射 |
| 仙逆1 | xianni | 男性，智慧长者，哲学且深刻 |
| 樊登极限 | fandenjixian | 男性，知识分子，洞察力强的演讲者 |
| 【宣传片】（大气悠扬浑厚） | xuanchuanpianda | 中性，宏大史诗，宣传式 |
| 贾小军终极版 | jiaxiaojunzong | 男性，热情推销员，快节奏 |
| 老女人快速版 | laonurenkuai | 女性，年长，紧急且说服力强 |
| 孙笑川258 | sunxiaochuan | 男性，悠闲，随意的游戏俚语 |
| 孙正聿（红色背景老头） | sunzhengyu | 男性，年长贤者，平静且建议性 |
| 温柔动听女声 | wenroudongting | 女性，柔软温柔，安抚ASMR |
| 陶矜 | taojin | 女性，专业，激励教育者 |
| 陶衿 | taojin | 女性，温暖，书卷气且反思 |
| 影视解说 | yingshijieshuo | 中性，叙述，悬念语气 |
| 女主播 | nvzhubo | 女性，活泼，宣传且生动 |
| 赛马娘（曼波欧耶版） | saimanian | 女性，精力充沛，歌唱式且有趣 |
| 贝利亚 | beiliya | 男性，反派，深沉且威胁 |
| 石斛片老中医 | shuhupianlao | 男性，年长治疗者，可信的推销 |
| 二狗 | ergou | 男性，兄弟式，建议且亲切 |
| 影视解说 | yingshijieshuo | 中性，批判，直率评论 |
| 易学 | yixue | 中性，耐心教师，有条理 |
| 仿真人（男声、偏用于广告、捏造事实类） | fanzhenren | 男性，炒作游戏玩家，夸张兴奋 |
| 琨哥 | kunge | 男性，友好兄弟，随意推荐 |
| 郭继承 | guojicheng | 男性，企业，专业且稳定 |
| 黑手 | heishou | 男性，粗犷街头，低沉且警告 |
| 祁同伟第二版 | qitongweidi | 男性，讲故事，讽刺且叙述 |
| 一鸣4 | yiming | 男性，分析游戏玩家，战略语气 |
| 大眼妹 | dayanmei | 女性，可爱影响者，活泼美容提示 |
| 张琦333 | zhangqi | 男性，随意评论者，实用且轻松 |
| 严格教师 | liuxiaoyan | 女性，严格教师，纪律且坚定 |
| 四川话 | yezi_sc | 四川话，女声 |
| 河南话 | xiaokun | 河南话，男声 |
| 温柔 | xiaowan | 普通话，女声 |

## 完整使用示例

### 示例 1：基础文本转语音

```bash
curl -X POST http://localhost:8000/v1/tts \
  -H "Content-Type: application/json" \
  -d '{
    "text": "欢迎使用 FishAudio 文本转语音服务。",
    "reference_id": "xiaowan",
    "format": "mp3"
  }'
```

### 示例 2：带情绪标签的文本

```bash
curl -X POST http://localhost:8000/v1/tts \
  -H "Content-Type: application/json" \
  -d '{
    "text": "今天是个好日子！(happy) 我们一起去庆祝吧！(excited)",
    "reference_id": "xiaowan",
    "temperature": 0.9,
    "top_p": 0.9,
    "format": "mp3"
  }'
```

### 示例 3：调整语速和音量

```bash
curl -X POST http://localhost:8000/v1/tts \
  -H "Content-Type: application/json" \
  -d '{
    "text": "这是一段需要慢速播放的文本。",
    "reference_id": "dongyuhui",
    "prosody": {
      "speed": 0.8,
      "volume": 5
    },
    "format": "wav"
  }'
```

### 示例 4：使用低延迟模式

```bash
curl -X POST http://localhost:8000/v1/tts \
  -H "Content-Type: application/json" \
  -d '{
    "text": "需要快速响应的文本内容。",
    "reference_id": "xiaowan",
    "latency": "balanced",
    "normalize": true,
    "format": "opus",
    "opus_bitrate": 48
  }'
```

### 示例 5：Python 客户端示例

```python
import requests
import json

url = "http://localhost:8000/v1/tts"
headers = {"Content-Type": "application/json"}

data = {
    "text": "你好，这是使用 Python 调用的示例。(happy)",
    "reference_id": "xiaowan",
    "temperature": 0.9,
    "top_p": 0.9,
    "prosody": {
        "speed": 1.0,
        "volume": 0
    },
    "format": "mp3",
    "mp3_bitrate": 128
}

response = requests.post(url, headers=headers, data=json.dumps(data))

if response.status_code == 200:
    with open("output.mp3", "wb") as f:
        f.write(response.content)
    print("音频文件已保存为 output.mp3")
else:
    print(f"错误: {response.status_code} - {response.text}")
```

## 注意事项

1. **温度参数**：`temperature` 和 `top_p` 参数影响生成语音的随机性和多样性，建议使用默认值 0.9 以获得最佳效果。

2. **块长度**：`chunk_length` 参数控制处理文本的块大小，范围在 100-300 之间。较大的值可能提高质量但增加延迟。

3. **格式选择**：
   - `wav`: 无损格式，文件较大
   - `mp3`: 有损压缩，文件较小，适合网络传输
   - `opus`: 高效压缩，适合实时应用
   - `pcm`: 原始音频数据

4. **延迟模式**：
   - `normal`: 标准模式，平衡质量和延迟
   - `balanced`: 低延迟模式，可能略微降低质量

5. **规范化**：启用 `normalize` 可以减少延迟，但可能影响数字和日期的发音准确性。

6. **情绪标签**：在文本中直接使用情绪标签，如 `(happy)`、`(sad)` 等，可以控制语音的情感表达。

## 技术支持

如有问题或建议，请联系技术支持团队。

