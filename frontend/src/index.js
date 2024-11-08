import React from 'react';
import ReactDOM from 'react-dom/client'; //   'react-dom'  'react-dom/client'
import App from './App';
import { BrowserRouter } from 'react-router-dom';
import './App.css';

const root = ReactDOM.createRoot(document.getElementById('root'));
root.render(
    <BrowserRouter>
        <App />
    </BrowserRouter>
);