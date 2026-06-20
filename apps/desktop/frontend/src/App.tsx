import { useState } from "react";
import { Greeting } from "./components/Greeting";
import { greet } from "./lib/desktop";
import { sanitizeName } from "./lib/name";

export default function App() {
  const [name, setName] = useState("");
  const [message, setMessage] = useState("");

  async function onGreet() {
    setMessage(await greet(sanitizeName(name)));
  }

  return (
    <main>
      <h1>Option Tab</h1>
      <input value={name} onChange={(e) => setName(e.target.value)} placeholder="Your name" />
      <button type="button" onClick={onGreet}>Greet</button>
      <Greeting message={message} />
    </main>
  );
}
