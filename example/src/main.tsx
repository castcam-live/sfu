import React, { useEffect } from "react";
import ReactDOM from "react-dom/client";
import "./index.css";
import { BrowserRouter, Routes, Route, useNavigate } from "react-router-dom";
import { Stream } from "./Stream.tsx";

function Redirect({ to }: { to: string }) {
	const navigate = useNavigate();
	useEffect(() => {
		navigate(to);
	}, []);
	return <></>;
}

function App() {
	return (
		<BrowserRouter>
			<Routes>
				<Route path="/stream" element={<Stream />} />
				<Route path="/receive" element={<div>Receive</div>} />
				<Route path="*" element={<Redirect to="/stream" />} />
			</Routes>
		</BrowserRouter>
	);
}

ReactDOM.createRoot(document.getElementById("root") as HTMLElement).render(
	<React.StrictMode>
		<App />
	</React.StrictMode>
);
