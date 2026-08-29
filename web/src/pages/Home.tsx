import { useEffect } from 'react'
import { Link } from 'react-router-dom'
import architectureUrl from '../assets/architecture.svg?url'
import { RevealTree } from '../components/RevealTree'
import { useAuth } from '../auth'
import {
  architectureSolutions,
  education,
  experienceTree,
  profile,
  projectTree,
  stackTree,
} from '../resume/data'
import { isAdmin } from '../types'

export function Home() {
  const { user } = useAuth()
  const showHermes = isAdmin(user)

  useEffect(() => {
    const id = window.location.hash.replace('#', '')
    if (!id) return
    document.getElementById(id)?.scrollIntoView({ block: 'start' })
  }, [])

  return (
    <>
      <section className="resume-card" aria-labelledby="profile-name">
        <div className="resume-card-main">
          <p className="kicker">Resume</p>
          <h1 id="profile-name">{profile.name}</h1>
          <p className="resume-role">{profile.title}</p>
          <p className="lede">{profile.lede}</p>
          <p className="resume-facts">
            <span>
              Experience: <em>{profile.years}</em>
            </span>
            <span>
              English: <em>{profile.english}</em>
            </span>
          </p>
        </div>
        <ul className="resume-contacts">
          {profile.contacts.map((item) => (
            <li key={item.label}>
              <span className="resume-contact-label">{item.label}</span>
              <a href={item.href}>{item.value}</a>
            </li>
          ))}
        </ul>
      </section>

      <nav className="resume-toc" aria-label="Resume sections">
        <a href="#stack">My Stack</a>
        <a href="#projects">Projects</a>
        <a href="#experience">Experience</a>
        <a href="#architecture">Architecture</a>
      </nav>

      <section className="resume-section" id="stack" aria-labelledby="stack-heading">
        <h2 id="stack-heading">My Stack</h2>
        <p className="resume-lead">The surface is what I ship. Open a branch for the libraries and the wiring.</p>
        <RevealTree items={stackTree} labelledBy="stack-heading" />
      </section>

      <section className="resume-section" id="projects" aria-labelledby="projects-heading">
        <h2 id="projects-heading">Projects</h2>
        <p className="resume-lead">Live Filenkov services first, then the contracts behind them.</p>
        <RevealTree items={projectTree(showHermes)} labelledBy="projects-heading" />
      </section>

      <section className="resume-section" id="experience" aria-labelledby="experience-heading">
        <h2 id="experience-heading">Experience</h2>
        <p className="resume-lead">Business numbers stay visible. Dig for the graph, the matrix, the protocol.</p>
        <RevealTree items={experienceTree} labelledBy="experience-heading" />

        <h3 className="resume-subhead">Education</h3>
        <ul className="edu-list">
          {education.map((item) => (
            <li key={item.school}>
              <div className="edu-row">
                <strong>{item.school}</strong>
                <span className="muted">{item.when}</span>
              </div>
              <p className="muted">{item.detail}</p>
            </li>
          ))}
        </ul>
      </section>

      <section className="resume-section" id="architecture" aria-labelledby="architecture-heading">
        <h2 id="architecture-heading">Architecture</h2>
        <p className="resume-lead">
          The house as it runs today. Each box will get its own page — start from a card below.
        </p>
        <figure className="arch-figure">
          <img
            className="arch-svg"
            src={architectureUrl}
            alt="Filenkov service map: Caddy and DNS in front of the auth gateway, product hosts, databases, Redis cache, MinIO, and gRPC validate calls."
          />
          <figcaption className="arch-caption">The live map. Scroll sideways if the screen is narrow.</figcaption>
        </figure>
        <ul className="arch-grid">
          {architectureSolutions.map((item) => (
            <li key={item.slug}>
              <Link className="arch-card" to={`/architecture/${item.slug}`}>
                <span className="arch-card-title">{item.title}</span>
                <span className="arch-card-summary">{item.summary}</span>
                <span className="arch-card-more">Solution page</span>
              </Link>
            </li>
          ))}
        </ul>
      </section>
    </>
  )
}
