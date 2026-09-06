## Contacts
Email: [fad80223@gmail.com](mailto:fad80223@gmail.com)
Telegram: [@alekseyfilenkov](https://t.me/alekseyfilenkov)
GitHub: [github.com/Aleshka5](https://github.com/Aleshka5)
Website: [filenkov.store](https://filenkov.store)
LinkedIn: [Alex Filenkov](https://www.linkedin.com/in/alex-filenkov-a50951433/)

## Education
**MAI (Moscow Aviation Institute), 2024 — 2026**
Data Science and Machine Learning. Dissertation: *Diffusion Transformer with Short-Term Memory*

**MPEI (National Research University "MPEI"), 2020 — 2024**
Bachelor's, Computer Systems and Engineering. Graduation thesis: *Telegram Bot with RAG System*

# My Experience
Data Science | Machine Learning | Python Backend | AI Agents

## Environment & Engineering Practices
**Linux:** Ubuntu is my daily driver for development, and I administer Ubuntu servers in production (systemd, apt, networking, Caddy/Docker hosts).
**Testing & Quality:** pytest with coverage, tests running in CI pipelines; static quality tooling — ruff, mypy, pre-commit.
**English:** B2 (Upper-Intermediate).

# Good Game — 08.2020 – 08.2021
Data Science | Machine Learning
Stack: TensorFlow, scikit-learn, Pandas, NumPy, Seaborn

This was a project where I built my own recommendation system for a streaming platform. I used Pandas and NumPy to preprocess data, and TensorFlow to build the machine learning solution.

## My Approach
I used TensorFlow and scikit-learn in my solution.

The data was quite messy, with a strong imbalance in the distribution of viewers across streamers — a handful of streamers accounted for most of the audience, while many others had only a small share of viewers. A naive approach would likely end up simply recommending the most popular streamers, so I decided not to use the streamer directly as a feature.

I had information about many streams, including: game, time, watcher ID, and when each viewer joined the stream. I visualized a lot of stream timelines and distributions using Matplotlib and Seaborn.

I split my solution into two parts:
1) Clustering by games
2) Prediction

First, I built two dictionaries:
1) For every streamer, which games or other topics they stream
2) For every watcher, which topics they watch

Another naive option would be to recommend only the topics a watcher already sees.

My approach was more interesting: I clustered streamers using K-nearest neighbors and DBSCAN. Then, for each watcher, I checked which streamers they had watched and pulled the most similar ones from that latent space.

**Advantages:**
- Very fast in production
- Easy to update for new streamers
- Handles new users well — you can suggest streamers from different clusters to quickly learn their preferences

## Other Approaches
- TensorFlow TFRS approach (poor interpretability)
- Collaborative filtering (lowest metrics in retrospective testing)

## Metrics and Results
- Over 80% Top-5 success rate in retrospective tests
- Strong matrix correlation

## Custom Metrics
During this project, I developed a new approach for interpreting recommendation system models, which clearly reveals model preferences. For details, see: https://github.com/Aleshka5/Matrix_correlation_for_streamers

# AeroClub — 08.2021 – 08.2023
Data Science | Machine Learning | Python Backend | LLM Pipelines
Stack: CatBoost, Pandas, Parquet, DVC, MLflow, scikit-learn Pipelines, RabbitMQ, LangChain

At AeroClub, I built two ML systems:

## Recommendation System
I built an ML-based recommendation system using a gradient boosting algorithm. There were thousands of features and hundreds of gigabytes of data, making it an ideal use case for this type of algorithm.

I used CatBoostRanker, the best-suited of CatBoost's algorithms for this task, with NDCG as the loss function since it's powerful and normalized.

I used MLflow, DVC, and GitHub together to track experiments and store models for production. The pipeline was:
1) Change the code
2) Store the dataset in DVC (S3-compatible data version control)
3) Commit the new data preprocessing to Git
4) Start an MLflow run (which references the Git commit, and the Git commit references the DVC version)

This gave me a fully reproducible experiment loop.

#### Metrics
- Reached 65% Top-1 and about 80% Top-10 accuracy from a cold start
- MRR ~70%

## LLM Solutions

### Multi-Label Classification — Intent Recognition
The next project was user intent recognition. A user would send us an email, and we needed to classify their intent in order to route the message to the right destination.

I didn't have enough data to train a new model, so I started with open-source solutions while collecting data:
1) Data collection with Label Studio + S3
2) Human-in-the-loop as a baseline
3) AI predicting intent with a confidence score

#### Metrics
Reached an average of 80% across all classes.

#### Custom Metrics
With each iteration, improving accuracy through prompt engineering meant finding pairs of similar intents and clearly separating them. I automated this using a confusion-matrix-based approach — though a standard confusion matrix doesn't directly apply to multi-label classification, so I had to adapt it.

*(A brief clarification: a confusion matrix counts TP, TN, FP, and FN and displays them as a matrix. It works well for binary and multi-class classification, but not for multi-label classification, where multiple classes can be predicted at once.)*

I adapted it for multi-label classification as follows. Imagine two sets:
- Predicted labels: {A, B, C, E}
- True labels: {A, B, D, F}

Two metrics are needed:
1) **Intersection** {A, B} → true positives
2) **Difference**: {C, E} vs. {D, F} → these are the confused/false positives

For example:
- Predicted {A, B}, True {A} → B is a false positive
- Predicted {A}, True {A, B} → B is a false negative

All other labels not present in either set are counted as true negatives.

### NER
When a message expressed intent to book a hotel, for example, we needed to convert the user's free text into a structured form for the hotel search system. We used Mistral Large as a baseline, hosted via vLLM, with LangChain to build the LLM processing chains.

#### Metrics
Over 60% precision across all fields (there were more than 20 fields).

#### Paged Document Scanning
Users could send us photos, Word documents, Excel files, and other formats. I wrote adapters for each file type and built paged-reading functions for the LLM, since our context window was quite small — reading page by page while carrying forward the NER results from previous pages.

## OCR
I set up an OCR service for photos and scanned documents, using EasyOCR as a baseline. It worked reasonably well for CPU inference but only achieved 50–60% accuracy, with a fair number of text errors.

So I built a more advanced pipeline:
OCR → bounding boxes → super-resolution for small, low-quality bounding boxes → a second OCR pass on the improved boxes → English and Russian spell-checking with correction.

This raised word-level precision to over 80% and significantly cut compute cost.

I also noticed that user photos frequently contained identical, low-value content — icons, business cards, and similar images. To reduce redundant compute, I added a Redis cache keyed by the hash of each photo, which cut over 50% of unnecessary processing.

## Python Backend
Other backend work included:
1) Email exchange integration
2) RabbitMQ for message routing
3) Label Studio for annotation and human-in-the-loop workflows
4) Microservice architecture

## LLM Framework
After several LLM services had been built and chained together, it became necessary to unify them into a single LLM framework for the overall business logic. I can't go into full detail due to an NDA, but it was a powerful framework connecting:
1) Label Studio
2) Annotation
3) LLM intent recognition
4) NER
5) OCR, and more

All of this ran together as one system, making the whole pipeline much easier and faster to improve. It was built following Clean Architecture principles.

# Sber — 08.2023 – 08.2026
Python Backend | AI Agents
Stack: Kafka, Async Programming, Vector DB (proprietary), RAG, LLM Agents (LangGraph), LangFuse, PostgreSQL, Memory, SQLAlchemy, Alembic

## Python Backend
I led the development team. We built an agent based on the LangGraph framework, along with a scheduler service. It runs with asynchronous concurrency, using a semaphore as a limiter.

I applied design patterns such as Factory, Singleton, Decorator, and Context Manager, along with both inheritance and composition where each was appropriate.

## LLM Testing
I set up LangFuse (with ClickHouse, Redis, and S3) together with LiteLLM to track LLM chains of thought. LiteLLM served as a single gateway for request routing and LLM API key distribution.

# My Own Website
Python Backend

I recently built my own portfolio website: https://filenkov.store

## Microservices Architecture
It includes:
1) Caddy proxy
2) API gateway written in Go
3) User microservice
4) Storage microservice
5) Self-hosted LLM with an open Hermes harness
6) Observability

Everything runs on Docker, with services separated across Docker networks to avoid collisions.

### Kubernetes
I deployed the website on Kubernetes (K8s) and used k9s to inspect logs and manage running workloads.

### HTTPS
I set up Caddy with Let's Encrypt to provide secure HTTPS connections.

## Storage (main service)
GitHub: https://github.com/Aleshka5/WebStorage
Link: https://storage.filenkov.store

### Gateway + Role Model
GitHub: https://github.com/Aleshka5/Auth-Service
Link: https://filenkov.store

### LLM Service
Link: https://llm.filenkov.store

### Hermes Plugins
Connects my gateway auth with Hermes auth, and updates the Hermes web UI.